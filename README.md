# meliodas

一个与OpenStack深度融合的cni插件

## 组件构成

- [tcnp-ipam](docs/ipam.md#)：基于neutron的cni ipam插件，提供三种模式

    1. separate模式：k8s、OpenStack分开部署，使用统一的neutron作为ip ipam
    2. mix模式：vm中部署k8s，pod同时运行ipvlan和[veth-host](docs/veth-host.md#)插件
    3. eni模式：使用网卡直通模式时用到

- [host-device](docs/host-device.md)：网卡直通到pod的cni插件，基于原生host-device插件改造
- ipvlan：官方原生的ipvlan cni插件
- [veth-host](docs/veth-host.md)：提供pod与宿主机建立veth-pair的cni插件
- [veth-route](docs/veth-route.md)：虚拟机vpc网络下，宿主机模拟dummy设备，pod通过dummy设备出网的cni插件，基于原生ptp插件改造
- veth-route-controller：自动刷新vrouter静态路由表的k8s controller，添加到pod的回包路由
- [meliodas-scheduler](docs/scheduler.md)：基于scheduler framework开发，根据虚拟机网卡辅助ip和虚拟机直通网卡剩余数量来调度的调度器
- [network-policy-controller](https://github.com/tkestack/galaxy/blob/master/doc/network-policy.md)：来自tkestack/galaxy, 基于ipset/iptables来实现

## 依赖的开源组件

- [ipvlan](https://github.com/containernetworking/plugins/blob/master/plugins/main/ipvlan/ipvlan.go)
- [host-device](https://github.com/containernetworking/plugins/blob/master/plugins/main/ptp/ptp.go)
- [ptp](https://github.com/containernetworking/plugins/blob/master/plugins/main/ptp/ptp.go)
- [gophercloud sdk](https://github.com/gophercloud/gophercloud)
