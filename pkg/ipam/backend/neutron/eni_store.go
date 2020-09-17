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
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/attachinterfaces"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/vishvananda/netlink"
	"log"
	"net"
	"os"
	"path/filepath"
)

// 弹性网卡直通模式
type EniStore struct {
	*FileLock
	NetworkClient *gophercloud.ServiceClient
	ComputeClient *gophercloud.ServiceClient
	InstanceID    string
	Networks      []string
}

// Store implements the Store interface
var _ backend.Store = &EniStore{}

func NewEniStore(ipamConfig *allocator.IPAMConfig) (backend.Store, error) {
	networkClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_NETWORK)
	if err != nil {
		return nil, err
	}
	if len(ipamConfig.NeutronConf.Networks) == 0 {
		return nil, fmt.Errorf("neutron networks is none")
	}

	computeClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_COMPUTE)
	if err != nil {
		return nil, err
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

	// find server
	server, err := FindServer(computeClient, hostAddrs[0].IP.String())
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf("can't find nova instance with ipv4 %s", hostAddrs[0].IP.String())
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
	store := &EniStore{
		NetworkClient: networkClient,
		ComputeClient: computeClient,
		Networks:      ipamConfig.NeutronConf.Networks,
		InstanceID:    server.ID,
		FileLock: lk,
	}
	return store, nil
}

// 申请弹性网卡，并返回ip信息
func (s *EniStore) Reserve(id string) (*net.IPNet, net.IP, error) {
	// 获取network对象
	network, err := networks.Get(s.NetworkClient, s.Networks[0]).Extract()
	if err != nil {
		return nil, nil, err
	}
	if len(network.Subnets) == 0 {
		return nil, nil, fmt.Errorf("neutron subnets is none")
	}

	// find port
	port := new(ports.Port)
	port, err = FindPort(s.NetworkClient, id)
	if err != nil {
		log.Printf("get neutron port %s err: %v", id, err.Error())
		return nil, nil, err
	}
	if port == nil {
		// port不存在，检测是否超过单vm网卡(设置为16，实际上CentOS 7单vm pci设备数量上限是32)
		// Reference: https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/7/html/virtualization_deployment_and_administration_guide/chap-guest_virtual_machine_device_configuration
		overMax, err := s.IsOverMax()
		if err != nil {
			return nil, nil, err
		}
		if *overMax {
			return nil, nil, fmt.Errorf("over single vm max supported enis:%d", backend.MAX_ENIS)
		}

		// port不存在，create port
		port, err = ports.Create(s.NetworkClient, ports.CreateOpts{
			NetworkID:    network.ID,
			Name:         id,
			Description:  getHostName(),
			AdminStateUp: backend.GetBoolPointer(true),
		}).Extract()
		if err != nil {
			log.Printf("create neutron port %s err: %v", id, err.Error())
			return nil, nil, err
		}
		log.Printf("created neutron port %s", port.ID)
	}

	if len(port.FixedIPs) == 0 {
		return nil, nil, fmt.Errorf("port doesn't have fixed ip")
	}

	subnetID := port.FixedIPs[0].SubnetID
	subnet, err := subnets.Get(s.NetworkClient, subnetID).Extract()
	if err != nil {
		log.Printf("get neutron subnet %s err: %v", subnetID, err.Error())
		return nil, nil, err
	}

	gw := net.ParseIP(subnet.GatewayIP)
	currentIP := net.ParseIP(port.FixedIPs[0].IPAddress)

	_, ipnet, err := net.ParseCIDR(subnet.CIDR)
	if err != nil {
		log.Printf("parse neutron subnet %s err: %v", subnetID, err.Error())
		return nil, nil, err
	}

	_, err = attachinterfaces.Create(s.ComputeClient, s.InstanceID, attachinterfaces.CreateOpts{
		PortID: port.ID,
	}).Extract()
	if err != nil {
		log.Printf("Instance %s create attachinterfaces err: %v", s.InstanceID, err.Error())
		return nil, nil, err
	}
	log.Printf("created nova attachinterface %s with instance %s", port.ID, s.InstanceID)
	return &net.IPNet{IP: currentIP, Mask: ipnet.Mask}, gw, nil
}

// 释放弹性网卡
func (s *EniStore) ReleaseByID(id string) error {
	port, err := FindPort(s.NetworkClient, id)
	if err != nil {
		log.Printf("get neutron port %s err: %v", id, err.Error())
		return err
	}
	if port == nil {
		log.Println("already deleted neutron port")
		return nil
	}
	log.Printf("found port %s", port.ID)

	// use neutron port delete api can also delete nova attachinterfaces
	err = ports.Delete(s.NetworkClient, port.ID).ExtractErr()
	if err != nil {
		return err
	}
	log.Printf("deleted neutron port %s", port.ID)
	return nil
}

func (s *EniStore) IsOverMax() (*bool, error) {
	attachInterfaces, err := attachinterfaces.List(s.ComputeClient, s.InstanceID).AllPages()
	if err != nil {
		log.Printf("Instance %s list attachinterfaces err: %v", s.InstanceID, err.Error())
		return nil, err
	}
	attachInterfacesSlice, err := attachinterfaces.ExtractInterfaces(attachInterfaces)
	if err != nil {
		return nil, err
	}
	if len(attachInterfacesSlice) >= backend.MAX_ENIS {
		return backend.GetBoolPointer(true), nil
	}
	return backend.GetBoolPointer(false), nil
}
