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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"log"
	"net"
	"sync"

	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	"github.com/yaoice/meliodas/pkg/ipam/backend"
)

// 分离模式，k8s与OpenStack分开部署
type SeparateStore struct {
	lock          *sync.RWMutex
	NetworkClient *gophercloud.ServiceClient
	Networks      []string
}

// Store implements the Store interface
var _ backend.Store = &SeparateStore{}

func NewSeparateStore(ipamConfig *allocator.IPAMConfig) (backend.Store, error) {
	networkClient, err := ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_NETWORK)
	if err != nil {
		return nil, err
	}
	if len(ipamConfig.NeutronConf.Networks) == 0 {
		return nil, fmt.Errorf("neutron networks is none")
	}
	// write values in Store object
	store := &SeparateStore{
		NetworkClient: networkClient,
		Networks:      ipamConfig.NeutronConf.Networks,
		lock: new(sync.RWMutex),
	}
	return store, nil
}

func (s *SeparateStore) Reserve(id string) (*net.IPNet, net.IP, error) {
	network, err := networks.Get(s.NetworkClient, s.Networks[0]).Extract()
	if err != nil {
		return nil, nil, err
	}
	if len(network.Subnets) == 0 {
		return nil, nil, fmt.Errorf("neutron subnets is none")
	}

	port, err := ports.Create(s.NetworkClient, ports.CreateOpts{
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

	return &net.IPNet{IP: currentIP, Mask: ipnet.Mask}, gw, nil
}

// N.B. This function eats errors to be tolerant and
// release as much as possible
func (s *SeparateStore) ReleaseByID(id string) error {
	// find port
	port := new(ports.Port)
	port, err := FindPort(s.NetworkClient, id)
	if err != nil {
		log.Printf("get neutron port %s err: %v", id, err.Error())
		return err
	}
	if err := ports.Delete(s.NetworkClient, port.ID).ExtractErr(); err != nil {
		log.Printf("delete neutron port %s err: %v", port.ID, err.Error())
		return err
	}
	log.Printf("deleted neutron port %s", port.ID)
	return nil
}

func (s *SeparateStore) Close() error {
	// stub we don't need close anything
	return nil
}

func (s *SeparateStore) Lock() error {
	s.lock.Lock()
	return nil
}

func (s *SeparateStore) Unlock() error {
	s.lock.Unlock()
	return nil
}
