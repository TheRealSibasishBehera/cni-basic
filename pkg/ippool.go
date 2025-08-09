package pkg

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

type IpPool struct {
	PoolName  string `json:"pool_name"`
	CidrRange string `json:"cidr_range"`
	Path      string `json:"path"`
	Gateway   int    `json:"gateway,omitempty"` //index from the allocation slice and not the actual IP

	TotalIps        int    `json:"total_ips"`
	LastAllocatedIP int    `json:"last_allocated_ip"`    // index from the Allocation slice and not the actual IP
	Allocation      []byte `json:"allocation,omitempty"` //allocated IPs in the pool
	//runtime state
	mutex sync.Mutex
}

func NewIpPool(poolName, cidrRange, path string) *IpPool {
	var ipPool IpPool
	if poolName == "" || cidrRange == "" || path == "" {
		log.Fatalf("Pool name, CIDR range, and path cannot be empty")
	}
	_, ipNet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		log.Fatalf("Invalid CIDR range: %s", cidrRange)
	}
	ipPool.PoolName = poolName
	ipPool.CidrRange = cidrRange
	ipPool.Path = path
	err = os.MkdirAll(path, 0755)
	if err != nil {
		log.Fatalf("Failed to create directory %s: %v", path, err)
	}

	// total - one for gateway , one for broadcast , one for network address
	// the gatewat would be used by the cni bridge plugin
	// representation is like
	// [0] = 0.2
	// [1] = 0.3
	// [2] = 0.4
	// [3] = 0.5
	// [4] = 0.6
	// [5] = 0.7
	// [6] = 0.8
	// lets say last allocated is 3
	// look from 2 to 0
	//if not free, look from 4 to 255
	size, _ := ipNet.Mask.Size()
	ipPool.TotalIps = 1<<uint(size) - 3

	ipPool.Allocation = make([]byte, ipPool.TotalIps)
	for i := 0; i < ipPool.TotalIps; i++ {
		ipPool.Allocation[i] = 0
	}
	return &ipPool
}

func (ipPool *IpPool) AllocateIP() (net.IP, error) {
	ipPool.mutex.Lock()
	defer ipPool.mutex.Unlock()

	if ipPool.LastAllocatedIP >= ipPool.TotalIps {
		return nil, fmt.Errorf("no more IPs available in the pool")
	}
	//reverse search
	// [0] is .2
	for i := ipPool.LastAllocatedIP; i >= 0; i-- {
		if ipPool.Allocation[i] == 0 {
			ipPool.Allocation[i] = 1
			// dont update last allocated for reverse search
			// ipPool.LastAllocatedIP = i
			ip := net.ParseIP(ipPool.CidrRange).To4()
			if ip == nil {
				return nil, fmt.Errorf("invalid CIDR range: %s", ipPool.CidrRange)
			}
			ip[3] += byte(i + 1)
			return ip, nil
		}
	}

	//forward search
	for i := ipPool.LastAllocatedIP + 1; i < ipPool.TotalIps; i++ {
		if ipPool.Allocation[i] == 0 {
			ipPool.Allocation[i] = 1
			ipPool.LastAllocatedIP = i
			ip := net.ParseIP(ipPool.CidrRange).To4()
			if ip == nil {
				return nil, fmt.Errorf("invalid CIDR range: %s", ipPool.CidrRange)
			}
			ip[3] += byte(i + 1)
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no available IPs in the pool")
}

func (ipPool *IpPool) ReleaseIP(ip net.IP) error {
	ipPool.mutex.Lock()
	defer ipPool.mutex.Unlock()

	if ip == nil {
		return fmt.Errorf("IP cannot be nil")
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return fmt.Errorf("IP is not a valid IPv4 address")
	}

	index := int(ip4[3]) - 1 // convert to index
	if index < 0 || index >= ipPool.TotalIps {
		return fmt.Errorf("IP %s is out of range for the pool", ip)
	}

	if ipPool.Allocation[index] == 0 {
		return fmt.Errorf("IP %s is not allocated", ip)
	}

	ipPool.Allocation[index] = 0
	if index == ipPool.LastAllocatedIP {
		for i := ipPool.LastAllocatedIP - 1; i >= 0; i-- {
			if ipPool.Allocation[i] == 1 {
				ipPool.LastAllocatedIP = i
				break
			}
		}
	}
	return nil
}

func (ipPool *IpPool) Save() error {
	ipPool.mutex.Lock()
	defer ipPool.mutex.Unlock()

	marshalledData, err := json.Marshal(ipPool)
	if err != nil {
		return fmt.Errorf("failed to marshal IP pool data: %v", err)
	}
	if err := os.WriteFile(ipPool.Path+"/"+ipPool.PoolName+".json", marshalledData, 0644); err != nil {
		return fmt.Errorf("failed to write data to file: %v", err)
	}
	return nil
}

func (ipPool *IpPool) Load() error {
	ipPool.mutex.Lock()
	defer ipPool.mutex.Unlock()

	data, err := os.ReadFile(ipPool.Path + "/" + ipPool.PoolName + ".json")
	if err != nil {
		return fmt.Errorf("failed to read IP pool data: %v", err)
	}

	if err := json.Unmarshal(data, ipPool); err != nil {
		return fmt.Errorf("failed to unmarshal IP pool data: %v", err)
	}
	return nil
}
