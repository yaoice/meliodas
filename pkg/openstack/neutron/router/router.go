package router

import (
	"encoding/json"
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/yaoice/meliodas/pkg/ipam/backend/neutron"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/vishvananda/netlink"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"net"
	"os"
	"path/filepath"
	"sync"
)

type NeutronRouter struct {
	lock *sync.RWMutex
	networkClient *gophercloud.ServiceClient
	subnet        *subnets.Subnet
	router        *routers.Router
}

func NewNeutronRouter() (*NeutronRouter, error) {
	var router *routers.Router
	ipamConfig, err := loadIpamConfig("/etc/cni/net.d/")
	if err != nil {
		return nil, err
	}
	networkClient, err := neutron.ConnectStore(ipamConfig.OpenStackConf, backend.SERVICE_TYPE_NETWORK)
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

	// find vrouter
	router, err = neutron.FindRouter(networkClient, subnet.GatewayIP)
	if err != nil {
		return nil, err
	}
	if router == nil {
		return nil, fmt.Errorf("can't find neutron vrouter with gateway ip %s", subnet.GatewayIP)
	}

	return &NeutronRouter{
		networkClient: networkClient,
		router:  router,
		subnet: subnet,
		lock: new(sync.RWMutex),
	}, nil
}

func loadIpamConfig(dataDir string) (*allocator.IPAMConfig, error) {
	var cniConf allocator.Net
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		err = json.Unmarshal(data, &cniConf)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if cniConf.IPAM.NeutronConf.HostInterface == "" {
		gwInterface, err := backend.GetRouteInterface()
		if err != nil {
			return nil, err
		}
		cniConf.IPAM.NeutronConf.HostInterface = *gwInterface
	}
	return cniConf.IPAM, nil
}

func(nr *NeutronRouter) UpdateRoutes(pods []*v1.Pod) error {
	nr.lock.Lock()
	defer nr.lock.Unlock()
	klog.Infof("Update router %s static routes start", nr.router.ID)

	routes, err := nr.parsePodsRoutes(pods)
	if err != nil {
		return err
	}
	klog.V(2).Infof("router %s static routes: [%v]", nr.router.ID, routes)
	_, err = routers.Update(nr.networkClient, nr.router.ID, routers.UpdateOpts{
		Routes:     routes,
	}).Extract()
	if err != nil {
		klog.Infof("Update router %s static routes err: %v", nr.router.ID, err)
		return err
	}
	klog.Infof("Update router %s static routes end", nr.router.ID)
	return nil
}

func(nr *NeutronRouter) parsePodsRoutes(pods []*v1.Pod) ([]routers.Route, error) {
	routes := make([]routers.Route, 0)
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			klog.Errorf("pod %s is in deleting status", pod.Name)
			continue
		}
		valid, err := nr.checkPodIP(pod.Status.PodIP)
		if err != nil {
			klog.Errorf("check pod %s ip err: %v", pod.Status.PodIP, err)
			continue
		}

		if *valid {
			route := routers.Route{
				NextHop:      pod.Status.HostIP,
				DestinationCIDR: fmt.Sprintf("%s/32", pod.Status.PodIP),
			}
			routes = append(routes, route)
		} else {
			klog.Infof("pod %s ip %s is not in router range %s", pod.Name, pod.Status.PodIP, nr.subnet.CIDR)
		}
	}
	return routes, nil
}

func(nr *NeutronRouter) checkPodIP(containerIP string) (*bool, error) {
	_, subnet, err := net.ParseCIDR(nr.subnet.CIDR)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(containerIP)
	return backend.GetBoolPointer(subnet.Contains(ip)), nil
}


