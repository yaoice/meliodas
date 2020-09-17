# meliodas

一个与OpenStack深度融合的cni插件，提供三种模式

1. separate模式
    
   k8s、OpenStack分开部署，使用统一的neutron作为[ipam](docs/host-device.md#)

2. mix模式
    
    vm中部署k8s，pod同时运行ipvlan和[veth-vpc](docs/host-device.md#)插件

3. eni模式
    
    虚拟机[弹性网卡直通](docs/host-device.md#)

## 依赖开源组件

- [ipvlan](https://github.com/containernetworking/plugins/blob/master/plugins/main/ipvlan/ipvlan.go)
- [host-device](https://github.com/containernetworking/plugins/blob/master/plugins/main/ptp/ptp.go)
- [ptp](https://github.com/containernetworking/plugins/blob/master/plugins/main/ptp/ptp.go)
- [gophercloud sdk](https://github.com/gophercloud/gophercloud)
