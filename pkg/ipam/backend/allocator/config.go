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

package allocator

import (
	"encoding/json"
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"net"

	"github.com/containernetworking/cni/pkg/types"
)

// The top-level network config - IPAM plugins are passed the full configuration
// of the calling plugin, not just the IPAM section.
type Net struct {
	Name          string      `json:"name"`
	CNIVersion    string      `json:"cniVersion"`
	IPAM          *IPAMConfig `json:"ipam"`
	RuntimeConfig struct {    // The capability arg
		IPRanges []RangeSet `json:"ipRanges,omitempty"`
	} `json:"runtimeConfig,omitempty"`
	Args *struct {
		A *IPAMArgs `json:"cni"`
	} `json:"args"`
}

// IPAMConfig represents the IP related network configuration.
// This nests Range because we initially only supported a single
// range directly, and wish to preserve backwards compatability
type IPAMConfig struct {
	*Range
	Name       string
	Type       string         `json:"type"`
	Routes     []*types.Route `json:"routes"`
	IPArgs     []net.IP       `json:"-"` // Requested IPs from CNI_ARGS and args
	ResolvConf string         `json:"resolvConf"`
	OpenStackConf *OpenStackConf `json:"openstackConf"`
	NeutronConf *NeutronConf  `json:"neutronConf"`
}

type OpenStackConf struct {
	UserName  string `json:"username"`
	PassWord  string `json:"password"`
	Project   string `json:"project"`
	Domain    string `json:"domain"`
	AuthUrl   string `json:"authUrl"`
}

type NeutronConf struct {
	Mode     string   `json:"mode"`
	DataDir  string   `json:"dataDir"`
	HostInterface string `json:"hostInterface"`
	Networks []string `json:"networks"`
}

type IPAMEnvArgs struct {
	types.CommonArgs
	IP net.IP `json:"ip,omitempty"`
}

type IPAMArgs struct {
	IPs []net.IP `json:"ips"`
}

type RangeSet []Range

type Range struct {
	RangeStart net.IP      `json:"rangeStart,omitempty"` // The first ip, inclusive
	RangeEnd   net.IP      `json:"rangeEnd,omitempty"`   // The last ip, inclusive
	Subnet     types.IPNet `json:"subnet"`
	Gateway    net.IP      `json:"gateway,omitempty"`
}

// NewIPAMConfig creates a NetworkConfig from the given network name.
func LoadIPAMConfig(bytes []byte, envArgs string) (*IPAMConfig, string, error) {
	n := Net{}
	if err := json.Unmarshal(bytes, &n); err != nil {
		return nil, "", err
	}

	if n.IPAM == nil {
		return nil, "", fmt.Errorf("IPAM config missing 'ipam' key")
	}

	// Parse custom IP from both env args *and* the top-level args config
	if envArgs != "" {
		e := IPAMEnvArgs{}
		err := types.LoadArgs(envArgs, &e)
		if err != nil {
			return nil, "", err
		}

		if e.IP != nil {
			n.IPAM.IPArgs = []net.IP{e.IP}
		}
	}

	if n.Args != nil && n.Args.A != nil && len(n.Args.A.IPs) != 0 {
		n.IPAM.IPArgs = append(n.IPAM.IPArgs, n.Args.A.IPs...)
	}

	for idx, _ := range n.IPAM.IPArgs {
		if err := canonicalizeIP(&n.IPAM.IPArgs[idx]); err != nil {
			return nil, "", fmt.Errorf("cannot understand ip: %v", err)
		}
	}

	if n.IPAM.NeutronConf == nil {
		return nil, "", fmt.Errorf("NeutronConf config missing 'neutronConf' key")
	}

	if len(n.IPAM.NeutronConf.Networks) == 0 {
		return nil, "", fmt.Errorf("NeutronConf config missing networks")
	}

	if n.IPAM.NeutronConf.HostInterface == "" {
		gwInterface, err := backend.GetRouteInterface()
		if err != nil {
			return nil, "", err
		}
		n.IPAM.NeutronConf.HostInterface = *gwInterface
	}

	if n.IPAM.NeutronConf.DataDir == "" {
		n.IPAM.NeutronConf.DataDir = backend.DefaultDataDir
	}

	// Copy net name into IPAM so not to drag Net struct around
	n.IPAM.Name = n.Name

	return n.IPAM, n.CNIVersion, nil
}

