package backend

import (
	"fmt"
	"github.com/vishvananda/netlink"
)

// get default gw interface
func GetRouteInterface() (*string, error) {
	var linkIndex *int
	routes, _ := netlink.RouteList(nil, netlink.FAMILY_V4)
	for _, r := range routes {
		if r.Dst == nil {
			linkIndex = &r.LinkIndex
			break
		}
	}
	if linkIndex == nil {
		return nil, fmt.Errorf("default gw route not set")
	}
	link, err:=netlink.LinkByIndex(*linkIndex)
	if err != nil {
		return nil, fmt.Errorf("get default gw route interface err: %v", err)
	}
	return &link.Attrs().Name, nil
}

// return bool pointer
func GetBoolPointer(b bool) *bool {
	return &b
}

