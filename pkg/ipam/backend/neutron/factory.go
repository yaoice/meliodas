package neutron

import (
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
)

func StoreFactory(ipamConfig *allocator.IPAMConfig) (backend.Store, error) {
	switch ipamConfig.NeutronConf.Mode {
	case "", backend.IPAM_MODE_SEPARATE:
		return NewSeparateStore(ipamConfig)
	case backend.IPAM_MODE_MIX_ROUTE:
		return NewMixRouteStore(ipamConfig)
	case backend.IPAM_MODE_MIX:
		return NewMixStore(ipamConfig)
	case backend.IPAM_MODE_ENI:
		return NewEniStore(ipamConfig)
	default:
		return nil, fmt.Errorf("unsupported mode")
	}
}
