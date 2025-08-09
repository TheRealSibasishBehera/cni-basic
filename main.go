package main

import (
	"encoding/json"
	"fmt"
	"os"

	"net"

	"github.com/TheRealSibasishBehera/cni-basic/pkg"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

// DEFAULT_MTU is the default MTU for the bridge interface
const DEFAULT_MTU = 1500

// NetworkConfig represents the CNI network configuration
type NetworkConfig struct {
	types.NetConf
	Bridge  string `json:"bridge,omitempty"`
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
	MTU     int    `json:"mtu,omitempty"`
	// IPMasq  bool   `json:"ipMasq,omitempty"`
	DNS *DNS `json:"dns,omitempty"`
}

type DNS struct {
	Nameservers []string `json:"nameservers,omitempty"`
}

// CNIResult represents the result returned by ADD command
type CNIResult struct {
	CNIVersion string         `json:"cniVersion"`
	Interfaces []CNIInterface `json:"interfaces,omitempty"`
	IPs        []CNIIP        `json:"ips,omitempty"`
	DNS        *types.DNS     `json:"dns,omitempty"`
}

// CNIInterface represents a network interface
type CNIInterface struct {
	Name    string `json:"name"`
	Mac     string `json:"mac,omitempty"`
	Sandbox string `json:"sandbox,omitempty"`
}

// CNIIP represents an IP assignment
type CNIIP struct {
	Interface int    `json:"interface,omitempty"`
	Address   string `json:"address"`
	Gateway   string `json:"gateway,omitempty"`
}

// CNIError represents an error response
type CNIError struct {
	CNIVersion string `json:"cniVersion"`
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	Details    string `json:"details,omitempty"`
}

// VersionResult represents the VERSION command response
type VersionResult struct {
	CNIVersion        string   `json:"cniVersion"`
	SupportedVersions []string `json:"supportedVersions"`
}

// Environment holds CNI environment variables
type Environment struct {
	Command     string
	ContainerID string
	NetNS       string
	IfName      string
	Args        string
	Path        string
	PID         int
}

// parseConfig parses the network configuration from stdin
func parseConfig() (*NetworkConfig, error) {
	var config NetworkConfig

	if err := json.NewDecoder(os.Stdin).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	if config.Bridge == "" {
		config.Bridge = "cni0"
	}
	if config.MTU == 0 {
		config.MTU = 1500
	}

	return &config, nil
}

// getEnvironment extracts CNI environment variables
func getEnvironment() *Environment {
	return &Environment{
		Command:     os.Getenv("CNI_COMMAND"),
		ContainerID: os.Getenv("CNI_CONTAINERID"),
		NetNS:       os.Getenv("CNI_NETNS"),
		IfName:      os.Getenv("CNI_IFNAME"),
		Args:        os.Getenv("CNI_ARGS"),
		Path:        os.Getenv("CNI_PATH"),
	}
}

// validateEnvironment checks required environment variables
func validateEnvironment(env *Environment) error {
	if env.Command == "" {
		return fmt.Errorf("CNI_COMMAND not set")
	}

	// For ADD/DEL/CHECK commands, these are required
	if env.Command != "VERSION" {
		if env.ContainerID == "" {
			return fmt.Errorf("CNI_CONTAINERID not set")
		}
		if env.NetNS == "" {
			return fmt.Errorf("CNI_NETNS not set")
		}
		if env.IfName == "" {
			return fmt.Errorf("CNI_IFNAME not set")
		}
	}

	return nil
}

// cmdAdd handles the ADD command
func cmdAdd(config *NetworkConfig, env *Environment) error {
	fmt.Fprintf(os.Stderr, "ADD command called for container %s\n", env.ContainerID)

	// TODO: Implement actual network setup
	// - Create/ensure bridge exists
	// - Create veth pair
	// - Move one end to container namespace
	// - Configure IP address via IPAM
	// - Set up routes

	// BRIDGE network setup if not present
	// ip link add name <bridge> type bridge

	var bridge netlink.Link
	var err error

	// create a new IP pool if not present (daemon operation in production)
	ipPool := pkg.NewIpPool(config.Subnet, config.Gateway, env.Path)
	ipPool.Save()

	if bridge, err = netlink.LinkByName(config.Bridge); err != nil {
		if err != nil {
			return fmt.Errorf("failed to get bridge %s: %v", config.Bridge, err)
		}
		if bridge == nil {
			link := &netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: config.Bridge,
					MTU:  config.MTU,
				},
			}
			if err := netlink.LinkAdd(link); err != nil {
				return fmt.Errorf("failed to create bridge %s: %v", config.Bridge, err)
			}
			if err := netlink.LinkSetUp(link); err != nil {
				return fmt.Errorf("failed to set up bridge %s: %v", config.Bridge, err)
			}
			fmt.Fprintf(os.Stderr, "Created bridge %s with MTU %d\n", config.Bridge, config.MTU)
		}
	}

	containerID := env.ContainerID
	strippedID := containerID[:5]

	// VETH pair setup
	// ifname lies in the container namespace
	// the veth-<containerID> will be the peer in the host namespace
	// its master would be the cni bridge
	var veth netlink.Link
	veth = &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: env.IfName,
			MTU:  config.MTU,
		},
		PeerName: fmt.Sprintf("veth-%s", strippedID),
	}

	// ip link add veth-<containerID> type veth peer name <ifname>
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("failed to create veth pair: %v", err)
	}

	// ip link set <ifname> netns <netns>
	containerVeth, err := netlink.LinkByName(env.IfName)
	err = netlink.LinkSetNsPid(containerVeth, env.PID)
	if err != nil {
		return fmt.Errorf("failed to move veth %s to namespace %s: %v", env.IfName, env.NetNS, err)
	}

	// ip link set veth-<containerID> master <bridge>
	peerVeth, err := netlink.LinkByName(fmt.Sprintf("veth-%s", strippedID))
	if err != nil {
		return fmt.Errorf("failed to get peer veth %s: %v", fmt.Sprintf("veth-%s", strippedID), err)
	}
	if err := netlink.LinkSetMaster(peerVeth, bridge); err != nil {
		return fmt.Errorf("failed to set master for peer veth %s: %v",
			fmt.Sprintf("veth-%s", strippedID), err)
	}

	// nsexec <netns> ip link set <ifname> up
	if err := netlink.LinkSetUp(containerVeth); err != nil {
		return fmt.Errorf("failed to set up veth %s in container namespace: %v", env.IfName, err)
	}

	// ip link set veth-<containerID> up
	if err := netlink.LinkSetUp(peerVeth); err != nil {
		return fmt.Errorf("failed to set up peer veth %s: %v", fmt.Sprintf("veth-%s", strippedID), err)
	}

	// I am assuming the lo is up already while contianer was being provisioned
	// lets give the gateway IP to the bridge
	_, ipNetwork, err := net.ParseCIDR(config.Subnet)
	if err != nil {
		return fmt.Errorf("failed to parse subnet %s: %v", config.Subnet, err)
	}

	// ip addr add <gatewayIP> dev <bridge>
	gatewayIP := net.ParseIP(config.Gateway)
	if gatewayIP == nil {
		return fmt.Errorf("invalid gateway IP %s", config.Gateway)
	}

	ones, bits := ipNetwork.Mask.Size()
	gatewayAddr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   gatewayIP,
			Mask: net.CIDRMask(ones, bits),
		},
	}
	if err := netlink.AddrAdd(bridge, gatewayAddr); err != nil {
		return fmt.Errorf("failed to add gateway IP %s to bridge %s: %v", config.Gateway, config.Bridge, err)
	}

	// lets give IP address to these
	// NOTE: the subnet might cause conflict with docker default driver
	// the config writeup should be used to avoid this

	// ip addr add <subnetIP> dev <ifname>
	if err := ipPool.Load(); err != nil {
		return fmt.Errorf("failed to load IP pool: %v", err)
	}
	var ip net.IP
	if ip, err = ipPool.AllocateIP(); err != nil {
		return fmt.Errorf("failed to allocate IP from pool: %v", err)
	}
	if err != nil {
		return fmt.Errorf("failed to allocate IP from pool: %v", err)
	}
	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(ones, bits),
		},
	}

	if err := netlink.AddrAdd(containerVeth, addr); err != nil {
		return fmt.Errorf("failed to add IP address %s to veth %s: %v", config.Subnet, env.IfName, err)
	}

	//to setup the dns servers,
	result := &CNIResult{
		CNIVersion: config.CNIVersion,
		Interfaces: []CNIInterface{
			{
				Name: config.Bridge,
				Mac:  bridge.Attrs().HardwareAddr.String(),
			},
			{
				Name:    env.IfName,
				Mac:     containerVeth.Attrs().HardwareAddr.String(),
				Sandbox: env.NetNS,
			},
		},
		IPs: []CNIIP{
			{
				Interface: 1,
				Address:   ip.String() + "/" + fmt.Sprintf("%d", ones),
				Gateway:   config.Gateway,
			},
		},
		DNS: &types.DNS{
			Nameservers: []string{"8.8.8.8", "1.1.1.1"},
		},
	}

	return json.NewEncoder(os.Stdout).Encode(result)
}

// cmdDel handles the DEL command
func cmdDel(config *NetworkConfig, env *Environment) error {
	fmt.Fprintf(os.Stderr, "DEL command called for container %s\n", env.ContainerID)

	// TODO: Implement actual network cleanup
	// - Remove interface from container
	// - Release IP address via IPAM
	// - Clean up bridge port if needed

	// DEL should return empty result on success

	return nil
}

// cmdCheck handles the CHECK command
func cmdCheck(config *NetworkConfig, env *Environment) error {
	fmt.Fprintf(os.Stderr, "CHECK command called for container %s\n", env.ContainerID)

	// TODO: Implement actual network validation
	// - Check if interface exists
	// - Verify IP configuration
	// - Validate connectivity

	// CHECK returns the same result as ADD if everything is correct
	result := &CNIResult{
		CNIVersion: config.CNIVersion,
		Interfaces: []CNIInterface{
			{
				Name:    env.IfName,
				Mac:     "00:11:22:33:44:55",
				Sandbox: env.NetNS,
			},
		},
		IPs: []CNIIP{
			{
				Interface: 0,
				Address:   "10.0.0.10/24",
				Gateway:   "10.0.0.1",
			},
		},
	}

	return json.NewEncoder(os.Stdout).Encode(result)
}

// cmdVersion handles the VERSION command
func cmdVersion() error {
	result := &VersionResult{
		CNIVersion:        "1.1.0",
		SupportedVersions: []string{"0.4.0", "1.0.0", "1.1.0"},
	}

	return json.NewEncoder(os.Stdout).Encode(result)
}

// sendError sends a formatted error response
func sendError(code int, msg string, details string, cniVersion string) {
	if cniVersion == "" {
		cniVersion = "1.1.0"
	}

	err := &CNIError{
		CNIVersion: cniVersion,
		Code:       code,
		Msg:        msg,
		Details:    details,
	}

	json.NewEncoder(os.Stdout).Encode(err)
}

func main() {
	// Get environment variables
	env := getEnvironment()

	// Validate environment
	if err := validateEnvironment(env); err != nil {
		sendError(4, "Invalid environment variables", err.Error(), "")
		os.Exit(1)
	}

	// Handle VERSION command separately (doesn't need config)
	if env.Command == "VERSION" {
		if err := cmdVersion(); err != nil {
			sendError(1, "Failed to get version", err.Error(), "")
			os.Exit(1)
		}
		return
	}

	// Parse network configuration
	config, err := parseConfig()
	if err != nil {
		sendError(7, "Invalid network configuration", err.Error(), "")
		os.Exit(1)
	}

	// Route to appropriate command handler
	switch env.Command {
	case "ADD":
		if err := cmdAdd(config, env); err != nil {
			sendError(7, "Failed to add network", err.Error(), config.CNIVersion)
			os.Exit(1)
		}
	case "DEL":
		if err := cmdDel(config, env); err != nil {
			sendError(7, "Failed to delete network", err.Error(), config.CNIVersion)
			os.Exit(1)
		}
	case "CHECK":
		if err := cmdCheck(config, env); err != nil {
			sendError(7, "Network check failed", err.Error(), config.CNIVersion)
			os.Exit(1)
		}
	default:
		sendError(4, "Unknown command", fmt.Sprintf("Command '%s' not supported", env.Command), config.CNIVersion)
		os.Exit(1)
	}
}
