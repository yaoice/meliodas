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
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/containernetworking/cni/pkg/types/current"
)

type IPAllocator struct {
	store backend.Store
}

func NewIPAllocator(store backend.Store) *IPAllocator {
	return &IPAllocator{
		store: store,
	}
}

// Get alocates an IP
func (a *IPAllocator) Get(id string) (*current.IPConfig, error) {
	a.store.Lock()
	defer a.store.Unlock()

	reservedIP, gw, err := a.store.Reserve(id)

	if err != nil {
		return nil, err
	}
	version := "4"
	if reservedIP.IP.To4() == nil {
		version = "6"
	}

	return &current.IPConfig{
		Version: version,
		Address: *reservedIP,
		Gateway: gw,
	}, nil
}

// Release clears all IPs allocated for the container with given ID
func (a *IPAllocator) Release(id string) error {
	a.store.Lock()
	defer a.store.Unlock()

	return a.store.ReleaseByID(id)
}
