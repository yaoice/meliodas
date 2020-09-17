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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

type NetConf struct {
	types.NetConf
	IPMasq bool `json:"ipMasq"`
	MTU    int  `json:"mtu"`
}

func setupContainerVeth(netns ns.NetNS, ifName string, mtu int, pr *current.Result) (*current.Interface, *current.Interface, error) {
	// The IPAM result will be something like IP=192.168.3.5/24, GW=192.168.3.1.
	// What we want is really a point-to-point link but veth does not support IFF_POINTOPONT.
	// Next best thing would be to let it ARP but set interface to 192.168.3.5/32 and
	// add a route like "192.168.3.0/24 via 192.168.3.1 dev $ifName".
	// Unfortunately that won't work as the GW will be outside the interface's subnet.

	// Our solution is to configure the interface with 192.168.3.5/24, then delete the
	// "192.168.3.0/24 dev $ifName" route that was automatically added. Then we add
	// "192.168.3.1/32 dev $ifName" and "192.168.3.0/24 via 192.168.3.1 dev $ifName".
	// In other words we force all traffic to ARP via the gateway except for GW itself.

	hostInterface := &current.Interface{}
	containerInterface := &current.Interface{}

	err := netns.Do(func(hostNS ns.NetNS) error {
		hostVeth, contVeth0, err := ip.SetupVeth(ifName, mtu, hostNS)
		if err != nil {
			return err
		}
		hostInterface.Name = hostVeth.Name
		hostInterface.Mac = hostVeth.HardwareAddr.String()
		containerInterface.Name = contVeth0.Name
		containerInterface.Mac = contVeth0.HardwareAddr.String()
		containerInterface.Sandbox = netns.Path()

		for _, ipc := range pr.IPs {
			// All addresses apply to the container veth interface
			ipc.Interface = current.Int(1)
		}

		pr.Interfaces = []*current.Interface{hostInterface, containerInterface}

		if err = ipam.ConfigureIface(ifName, pr); err != nil {
			return err
		}

		contVeth, err := net.InterfaceByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to look up %q: %v", ifName, err)
		}

		// delete all route at first
		routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("failed to list route: %v", err)
		}
		for _, route := range routes {
				if err := netlink.RouteDel(&route); err != nil {
					return fmt.Errorf("failed to delete route %v: %v", route, err)
				}
		}

		for _, ipc := range pr.IPs {
			addrBits := 32
			if ipc.Version == "6" {
				addrBits = 128
			}

			_, defaultGwCidr, err := net.ParseCIDR("0.0.0.0/0")
			if err != nil {
				return fmt.Errorf("failed to parse default gw cidr :%v", err)
			}

			for _, r := range []netlink.Route{
				netlink.Route{
					LinkIndex: contVeth.Index,
					Dst: &net.IPNet{
						IP:   net.ParseIP("169.254.1.1"),
						Mask: net.CIDRMask(addrBits, addrBits),
					},
					Scope: netlink.SCOPE_LINK,
					Src:   ipc.Address.IP,
				},
				netlink.Route{
					LinkIndex: contVeth.Index,
					Dst: defaultGwCidr,
					Scope: netlink.SCOPE_UNIVERSE,
					Gw:    net.ParseIP("169.254.1.1"),
				},
			} {
				if err := netlink.RouteAdd(&r); err != nil {
					return fmt.Errorf("failed to add route %v: %v", r, err)
				}
			}
		}

		// Send a gratuitous arp for all v4 addresses
		for _, ipc := range pr.IPs {
			if ipc.Version == "4" {
				_ = arping.GratuitousArpOverIface(ipc.Address.IP, *contVeth)
			}
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return hostInterface, containerInterface, nil
}

func setupHostVeth(vethName string, result *current.Result) error {
	// hostVeth moved namespaces and may have a new ifindex
	veth, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", vethName, err)
	}

	for _, ipc := range result.IPs {
		maskLen := 128
		if ipc.Address.IP.To4() != nil {
			maskLen = 32
		}

		ipn := &net.IPNet{
			IP:   ipc.Address.IP,
			Mask: net.CIDRMask(maskLen, maskLen),
		}
		// dst happens to be the same as IP/net of host veth
		if err = ip.AddRoute(ipn, nil, veth); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to add route on host: %v", err)
		}
	}

	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	conf := NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	// run the IPAM plugin and get back the config to apply
	r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}
	// Convert whatever the IPAM result was into the current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return err
	}

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}

	if err := ip.EnableForward(result.IPs); err != nil {
		return fmt.Errorf("Could not enable IP forwarding: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	// ensure to create device with ip 169.254.1.1
	if err := setupDummy(); err != nil {
		return err
	}

	hostInterface, containerInterface, err := setupContainerVeth(netns, args.IfName, conf.MTU, result)
	if err != nil {
		return err
	}

	if err = setupHostVeth(hostInterface.Name, result); err != nil {
		return err
	}

	if conf.IPMasq {
		chain := utils.FormatChainName(conf.Name, args.ContainerID)
		comment := utils.FormatComment(conf.Name, args.ContainerID)
		for _, ipc := range result.IPs {
			if err = ip.SetupIPMasq(&ipc.Address, chain, comment); err != nil {
				return err
			}
		}
	}

	result.DNS = conf.DNS
	result.Interfaces = []*current.Interface{hostInterface, containerInterface}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	conf := NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	if err := ipam.ExecDel(conf.IPAM.Type, args.StdinData); err != nil {
		return err
	}

	if args.Netns == "" {
		return nil
	}

	// There is a netns so try to clean up. Delete can be called multiple times
	// so don't return an error if the device is already removed.
	// If the device isn't there then don't try to clean up IP masq either.
	var ipnets []*net.IPNet
	err := ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		var err error
		ipnets, err = ip.DelLinkByNameAddr(args.IfName)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		return err
	})

	if err != nil {
		return err
	}

	if len(ipnets) != 0 && conf.IPMasq {
		chain := utils.FormatChainName(conf.Name, args.ContainerID)
		comment := utils.FormatComment(conf.Name, args.ContainerID)
		for _, ipn := range ipnets {
			err = ip.TeardownIPMasq(ipn, chain, comment)
		}
	}

	return err
}

func setupDummy() error {
	dummyName := "dummy0"
	// create dummy if necessary
	dummy, err := ensureDummy(dummyName)
	if err != nil {
		return fmt.Errorf("failed to create dummy %q: %v", dummyName, err)
	}
	maskLen := 32

	ipn := &net.IPNet{
		IP:   net.ParseIP("169.254.1.1"),
		Mask: net.CIDRMask(maskLen, maskLen),
	}
	if err := ensureAddr(dummy, netlink.FAMILY_ALL, ipn, false); err != nil {
		return err
	}
	return nil
}

func ensureDummy(dummyName string) (*netlink.Dummy, error) {
	dummy := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
		Name:         dummyName,
	}}

	err := netlink.LinkAdd(dummy)
	if err != nil && err != syscall.EEXIST {
		return nil, fmt.Errorf("could not add %q: %v", dummyName, err)
	}

	// Re-fetch link to read all attributes and if it already existed,
	// ensure it's really a dummy with similar configuration
	dummy, err = dummyByName(dummyName)
	if err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(dummy); err != nil {
		return nil, err
	}

	// ensure to set dummy promisc
	if err := netlink.SetPromiscOn(dummy); err != nil {
		return nil, fmt.Errorf("could not set promiscuous mode on %q: %v", dummyName, err)
	}

	return dummy, nil
}

func dummyByName(name string) (*netlink.Dummy, error) {
	l, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}
	dummy, ok := l.(*netlink.Dummy)
	if !ok {
		return nil, fmt.Errorf("%q already exists but is not a dummy", name)
	}
	return dummy, nil
}

func ensureAddr(dummy netlink.Link, family int, ipn *net.IPNet, forceAddress bool) error {
	addrs, err := netlink.AddrList(dummy, family)
	if err != nil && err != syscall.ENOENT {
		return fmt.Errorf("could not get list of IP addresses: %v", err)
	}

	ipnStr := ipn.String()
	for _, a := range addrs {

		// string comp is actually easiest for doing IPNet comps
		if a.IPNet.String() == ipnStr {
			return nil
		}

		// Multiple IPv6 addresses are allowed on the bridge if the
		// corresponding subnets do not overlap. For IPv4 or for
		// overlapping IPv6 subnets, reconfigure the IP address if
		// forceAddress is true, otherwise throw an error.
		if family == netlink.FAMILY_V4 || a.IPNet.Contains(ipn.IP) || ipn.Contains(a.IPNet.IP) {
			if forceAddress {
				if err = deleteAddr(dummy, a.IPNet); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%q already has an IP address different from %v", dummy.Attrs().Name, ipnStr)
			}
		}
	}

	addr := &netlink.Addr{IPNet: ipn, Label: ""}
	if err := netlink.AddrAdd(dummy, addr); err != nil && err != syscall.EEXIST {
		return fmt.Errorf("could not add IP address to %q: %v", dummy.Attrs().Name, err)
	}

	// Set the bridge's MAC to itself. Otherwise, the bridge will take the
	// lowest-numbered mac on the bridge, and will change as ifs churn
	if err := netlink.LinkSetHardwareAddr(dummy, dummy.Attrs().HardwareAddr); err != nil {
		return fmt.Errorf("could not set dummy's mac: %v", err)
	}

	return nil
}

func deleteAddr(dummy netlink.Link, ipn *net.IPNet) error {
	addr := &netlink.Addr{IPNet: ipn, Label: ""}

	if err := netlink.AddrDel(dummy, addr); err != nil {
		return fmt.Errorf("could not remove IP address from %q: %v", dummy.Attrs().Name, err)
	}

	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.All)
}