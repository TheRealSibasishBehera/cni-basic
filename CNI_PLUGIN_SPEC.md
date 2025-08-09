# Basic CNI Plugin Specification

## Overview
This document outlines the specification for a basic Container Network Interface (CNI) plugin that provides fundamental networking capabilities for containers.

## Plugin Metadata
- **Name**: cni-basic
- **Version**: 1.0.0
- **CNI Spec Version**: 1.1.0
- **Type**: Interface plugin

## Supported Operations

### ADD
Creates and configures a network interface inside the container.

**Environment Variables:**
- `CNI_COMMAND=ADD`
- `CNI_CONTAINERID`: Unique container identifier
- `CNI_NETNS`: Path to container's network namespace
- `CNI_IFNAME`: Interface name to create (default: eth0)
- `CNI_ARGS`: Additional arguments (optional)
- `CNI_PATH`: Plugin search paths

**Input Configuration:**
```json
{
  "cniVersion": "1.1.0",
  "name": "basic-network",
  "type": "cni-basic",
  "bridge": "cni0",
  "subnet": "10.0.0.0/24",
  "gateway": "10.0.0.1",
  "ipam": {
    "type": "host-local",
    "subnet": "10.0.0.0/24",
    "gateway": "10.0.0.1"
  },
  "dns": {
    "nameservers": ["8.8.8.8", "1.1.1.1"]
  }
}
```

**Output (Success):**
```json
{
  "cniVersion": "1.1.0",
  "interfaces": [
    {
      "name": "cni0",
      "mac": "aa:bb:cc:dd:ee:ff"
    },
    {
      "name": "eth0",
      "mac": "00:11:22:33:44:55",
      "sandbox": "/var/run/netns/container123"
    }
  ],
  "ips": [
    {
      "interface": 1,
      "address": "10.0.0.10/24",
      "gateway": "10.0.0.1"
    }
  ],
  "dns": {
    "nameservers": ["8.8.8.8", "1.1.1.1"]
  }
}
```

### DEL
Removes the network interface and cleans up allocated resources.

**Environment Variables:**
- `CNI_COMMAND=DEL`
- Same variables as ADD operation

**Expected Behavior:**
- Delete container interface
- Remove IP allocation
- Clean up bridge port if no longer needed
- Return empty result on success

### CHECK
Validates the current state of the network interface.

**Environment Variables:**
- `CNI_COMMAND=CHECK`
- Same variables as ADD operation

**Expected Behavior:**
- Verify interface exists and is configured correctly
- Check IP assignment matches expected configuration
- Return error if inconsistencies found

### VERSION
Returns supported CNI specification versions.

**Output:**
```json
{
  "cniVersion": "1.1.0",
  "supportedVersions": ["0.4.0", "1.0.0", "1.1.0"]
}
```

## Configuration Parameters

### Required Fields
- `cniVersion`: CNI specification version
- `name`: Network name (must be unique)
- `type`: Plugin binary name ("cni-basic")

### Optional Fields
- `bridge`: Bridge interface name (default: "cni0")
- `subnet`: Network subnet in CIDR notation
- `gateway`: Gateway IP address
- `mtu`: Maximum transmission unit (default: 1500)
- `ipMasq`: Enable IP masquerading (default: false)
- `ipam`: IPAM plugin configuration
- `dns`: DNS configuration

## Network Architecture

### Bridge-based Networking
The plugin creates a simple bridge-based network topology:

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Container A   │    │    Host      │    │   Container B   │
│                 │    │              │    │                 │
│ eth0: 10.0.0.10 │────│ cni0 bridge  │────│ eth0: 10.0.0.11 │
└─────────────────┘    │ 10.0.0.1     │    └─────────────────┘
                       └──────────────┘
```

### IP Address Management
- Uses host-local IPAM for IP allocation by default
- Supports static IP assignment via configuration
- Maintains IP allocation state in `/var/lib/cni/networks/<network-name>/`

## Error Handling

### Error Response Format
```json
{
  "cniVersion": "1.1.0",
  "code": 7,
  "msg": "Invalid network configuration",
  "details": "Bridge interface 'invalid-bridge' does not exist"
}
```

### Common Error Codes
- `1`: Incompatible CNI version
- `2`: Unsupported field in network configuration
- `3`: Container unknown or does not exist
- `4`: Invalid environment variables
- `7`: Invalid network configuration
- `11`: Try again later

## Implementation Requirements

### Dependencies
- Linux bridge utilities (`brctl` or `ip link`)
- Network namespace utilities (`ip netns`)
- IPAM plugin (host-local recommended)

### File Locations
- Plugin binary: `/opt/cni/bin/cni-basic`
- Network configurations: `/etc/cni/net.d/`
- State data: `/var/lib/cni/networks/`

### Security Considerations
- Plugin must run with appropriate privileges (CAP_NET_ADMIN)
- Validate all input parameters
- Sanitize network namespace paths
- Implement proper error handling and cleanup

## Testing Strategy

### Unit Tests
- Configuration parsing
- IP address validation
- Bridge operations
- Namespace handling

### Integration Tests
- Full ADD/DEL/CHECK workflow
- Multi-container scenarios
- IPAM integration
- Error conditions

### Compliance Tests
- CNI specification conformance
- Kubernetes integration testing
- Performance benchmarks

## Future Extensions

### Planned Features
- Support for multiple bridge interfaces
- VLAN tagging capabilities
- QoS and bandwidth limiting
- IPv6 support
- Custom routing rules

### Plugin Chaining Support
The plugin is designed to work in chain with other CNI plugins:
- Supports `prevResult` input for chained execution
- Can be combined with firewall, QoS, or monitoring plugins
- Maintains interface information for downstream plugins

### IPPool Tracking 

- calculate the number of IP address available 
- store the last allocated IP 
- maintain a Array with 0 for not allocated and 1 for allocated 
- when deleting set to 0
- when allocating set to 1 
- search while allocation 0(n) search
  - start with lastAllocatedIp = -1 
  - <-- search all elements from lastAllocatedIP to 0
  - if you get a value , use it , set it to 1 
  - else use lastAllocatedIp + 1 , lastAllocated = lastAllocated + 1
- we need a persistent location to store this datastructure 
