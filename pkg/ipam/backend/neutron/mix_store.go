// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package neutron

import (
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/vishvananda/netlink"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yaoice/meliodas/pkg/ipam/backend"
)

// 融合模式，弹性网卡多IP
type MixStore struct {
	*FileLock

	NetworkClient *gophercloud.ServiceClient
	hostPortID    string
	subnet        *subnets.Subnet
	network       *networks.Network
	dataDir       string
}

// Store implements the Store interface
var _ backend.Store = &MixStore{}

func NewMixStore(ipamConfig *allocator.IPAMConfig) (backend.Store, error) {
	networkClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_NETWORK)
	if err != nil {
		return nil, err
	}
	if len(ipamConfig.NeutronConf.Networks) == 0 {
		return nil, fmt.Errorf("neutron networks is none")
	}

	iface, err := netlink.LinkByName(ipamConfig.NeutronConf.HostInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup %q: %v", ipamConfig.NeutronConf.HostInterface, err)
	}

	hostAddrs, err := netlink.AddrList(iface, netlink.FAMILY_ALL)
	if err != nil || len(hostAddrs) == 0 {
		return nil, fmt.Errorf("failed to get host IP addresses for %q: %v", iface, err)
	}
	log.Printf("host address: %s", hostAddrs[0].IP.String())

	network, err := networks.Get(networkClient, ipamConfig.NeutronConf.Networks[0]).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get neutron network object: %v", err)
	}
	if len(network.Subnets) == 0 {
		return nil, fmt.Errorf("neutron network %s subnets is none", ipamConfig.NeutronConf.Networks[0])
	}

	hostPorts, err := ports.List(networkClient, ports.ListOpts{
		// To list all networks
		// NetworkID:    network.ID,
		FixedIPs:     []ports.FixedIPOpts{
			{
				IPAddress: hostAddrs[0].IP.String(),
			},
		},
	}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("get host neutron port err: %v", err)
	}

	hostPortsSlice, err := ports.ExtractPorts(hostPorts)
	if err != nil {
		return nil, err
	}

	if len(hostPortsSlice) == 0 {
		return nil, fmt.Errorf("failed to get host neutron port")
	}

	if len(hostPortsSlice[0].FixedIPs) == 0 {
		return nil, fmt.Errorf("failed to get host neutron port fixed ip")
	}

	subnet, err := subnets.Get(networkClient, hostPortsSlice[0].FixedIPs[0].SubnetID).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to get host neutron subnet %s: %v", hostPortsSlice[0].FixedIPs[0].SubnetID, err)
	}

	dir := filepath.Join(ipamConfig.NeutronConf.DataDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	lk, err := NewFileLock(dir)
	if err != nil {
		return nil, err
	}

	// write values in Store object
	store := &MixStore{
		FileLock: lk,
		NetworkClient: networkClient,
		network:      network,
		hostPortID: hostPortsSlice[0].ID,
		subnet: subnet,
		dataDir: ipamConfig.NeutronConf.DataDir,
	}
	return store, nil
}

func (s *MixStore) Reserve(id string) (*net.IPNet, net.IP, error) {
	port, err := ports.Get(s.NetworkClient, s.hostPortID).Extract()
	if err != nil {
		log.Printf("get host neutron port %s err: %v", s.hostPortID, err)
		return nil, nil, err
	}

	overMax, err := s.IsOverMax(port)
	if err != nil {
		return nil, nil, err
	}
	if *overMax {
		return nil, nil, fmt.Errorf("over single network interface max supported ips:%d", backend.MAX_IPS)
	}

	oldFixedIPsSlice := port.FixedIPs
	port.FixedIPs = append(port.FixedIPs, ports.IP{
		SubnetID:  s.subnet.ID,
	})

	port, err = ports.Update(s.NetworkClient, s.hostPortID, ports.UpdateOpts{
		FixedIPs: port.FixedIPs,
	}).Extract()
	if err != nil {
		log.Printf("add host neutron port %s err: %v", s.hostPortID, err)
		return nil, nil, err
	}
	log.Printf("updated neutron port %s: %v", s.hostPortID, port.FixedIPs)
	newFixedIPsSlice := port.FixedIPs
	newIP := difference(oldFixedIPsSlice, newFixedIPsSlice)
	if newIP == nil {
		return nil, nil, fmt.Errorf("port doesn't have new fixed ip")
	}
	log.Printf("new neutron port fixed ip: %s", newIP.IPAddress)
	gw := net.ParseIP(s.subnet.GatewayIP)
	currentIP := net.ParseIP(newIP.IPAddress)

	_, ipnet, err := net.ParseCIDR(s.subnet.CIDR)
	if err != nil {
		log.Printf("parse neutron subnet %s err: %v", s.subnet.ID, err.Error())
		return nil, nil, err
	}
	fname := GetEscapedPath(s.dataDir, currentIP.String())
	f, err := os.OpenFile(fname, os.O_RDWR|os.O_EXCL|os.O_CREATE, 0644)
	if os.IsExist(err) {
		return nil, nil, err
	}
	if err != nil {
		return nil, nil, err
	}
	if _, err = f.WriteString(strings.TrimSpace(id)); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, nil, err
	}
	if err = f.Close(); err != nil {
		os.Remove(f.Name())
		return nil, nil, err
	}
	return &net.IPNet{IP: currentIP, Mask: ipnet.Mask}, gw, nil
}

// N.B. This function eats errors to be tolerant and
// release as much as possible
func (s *MixStore) ReleaseByID(id string) error {
	var (
		containerIP string
		containerIPPath string
	)
	filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.TrimSpace(string(data)) == strings.TrimSpace(id) {
			containerIP = info.Name()
			containerIPPath = path
			return fmt.Errorf("found container ip")
		}
		return nil
	})

	if containerIP == "" {
		log.Printf("container %s ip not found", id)
		return nil
	}
	port, err := ports.Get(s.NetworkClient, s.hostPortID).Extract()
	if err != nil {
		log.Printf("get host neutron port %s err: %v", s.hostPortID, err)
		return err
	}
	if index := s.findIndex(port.FixedIPs, containerIP); index != nil {
	    port.FixedIPs = append(port.FixedIPs[:*index], port.FixedIPs[*index+1:]...)
		_, err = ports.Update(s.NetworkClient, s.hostPortID, ports.UpdateOpts{
			FixedIPs: port.FixedIPs,
		}).Extract()
		if err != nil {
			log.Printf("delete host neutron port %s err: %v", s.hostPortID, err)
			return err
		}
		log.Printf("updated neutron port %s: %v", s.hostPortID, port.FixedIPs)
	}

	if exists(containerIPPath) {
		if err := os.Remove(containerIPPath); err != nil {
			return err
		}
	}
	return nil
}

func (s *MixStore) findIndex(ips []ports.IP, targetIP string) *int {
	for index, ip := range ips {
		if ip.IPAddress == targetIP {
			return &index
		}
	}
	return nil
}

func (s *MixStore) IsOverMax(port *ports.Port) (*bool, error) {
	if len(port.FixedIPs) >= backend.MAX_IPS {
		return backend.GetBoolPointer(true), nil
	}
	return backend.GetBoolPointer(false), nil
}

func GetEscapedPath(dataDir string, fname string) string {
	if runtime.GOOS == "windows" {
		fname = strings.Replace(fname, ":", "_", -1)
	}
	return filepath.Join(dataDir, fname)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func difference(oldS, newS []ports.IP) *ports.IP {
	log.Printf("old fixed ips: %v", oldS)
	log.Printf("new fixed ips: %v", newS)
	if len(newS) <= len(oldS) {
		return nil
	}
	for _, newIP := range newS {
		flag := false
		for _, oldIP := range oldS {
			if newIP.IPAddress == oldIP.IPAddress && newIP.SubnetID == oldIP.SubnetID {
				flag = true
			}
		}
		if !flag {
			return &newIP
		}
	}
	return nil
}

