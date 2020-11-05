## 适用场景
当使用macvlan、ipvlan、sriov等underlay网络方案时，如遇到pod与宿主机IP不通或无法访问serviceIP，可配合使用该cni插件有效解决问题。

## 使用方法

* 将本cni插件的二进制文件放入`CNI_BIN_PATH`。

* 正确配置cni的配置文件，如下：

  ```bash
  $ cat /etc/cni/net.d/10-cnichain.conflist
  {
    "name": "cni0",
    "cniVersion": "0.3.1",
    "plugins": [
        {
            "type": "ipvlan",
            "master": "enp1s0",
            "ipam": {
                "type": "tcnp-ipam",
                "openstackConf": {
                   "username": "admin",
                   "password": "b974a1991171",
                   "project": "admin",
                   "domain": "default",
                   "authUrl": "http://192.168.55.250:35357/v3"
                },
                "neutronConf": {
                   "networks": ["4ae3f89d-b41a-4fd1-99d3-6a0861182946"],
                   "mode": "mix"
                },
                "routes": [{
                    "dst": "0.0.0.0/0"
                }]
            }
        },
        {
            "type": "veth-host",
            "serviceCidr": "172.20.252.0/22",
            "hostInterface": "enp1s0",
            "containerInterface": "veth0",
            "ipMasq": true
        }
    ]
  }
  ```
  
## 插件原理

* 创建⼀对`veth pair`，⼀端挂⼊容器内，⼀端挂⼊宿主机内。 
* 在容器内设置路由规则，当目标地址是宿主机IP或非underlay网络，则使用veth将数据包转出，nexthop地址是宿主机IP。
* 在宿主机内设置路由规则，当目标地址是容器的underlay网络IP时，使用veth将数据包转出, nexthop地址是容器的underlay网络IP。

## 插件选项

| 序号    |   选项名称   |   含义   |   默认值   |
| ---- | ---- | ---- | ---- |
|   1    | hostInterface | 宿主机的网络接口名，一般是underlay网络的parent接口 | 无 |
|   2    | containerInterface | 插入容器的veth网络接口名，一般设置为veth0 | 无 |
|   3   | ipMasq | 是否设置IP masquerade规则 | false |
| 4 | mtu | 网络接口的MTU | 1500 |
| 5 | routeTableStart | 在宿主机上插入自定义路由表时，路由表的起始ID，分配的路由表ID将大于该设置 | 256 |
| 6 | nodePortMark | kubernets里标识nodeport流量时采用的mark | 0x2000 |
| 7 | nodePorts | nodeport的端口范围，这个要与kube-apiserver里的service-node-port-range设置一致 | 30000:32767 |

## 诊断方法

可以通过检查该方案创建的网络接口、路由规则等诊断该CNI插件是否正常工作, 检查方法如下：

```bash
# 检查pod容器里的网络接口
[root@node-1 ~]# kubectl -n demo exec -ti redis-predixy-67d989bdd9-p7fbf sh
/ # ip link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: eth0@if3: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UNKNOWN
    link/ether 2e:31:a0:bc:39:20 brd ff:ff:ff:ff:ff:ff
4: veth0@if139: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1500 qdisc noqueue state UP
    link/ether 46:4a:fd:d5:3a:bf brd ff:ff:ff:ff:ff:ff
# 检查pod容器里的路由规则
/ # ip route show
default via 10.10.20.152 dev veth0
10.10.20.0/24 dev eth0 scope link  src 10.10.20.53
10.10.20.152 dev veth0 scope link
# 退出pod容器
/ # exit

# 检查宿主机里的网络接口
[root@node-1 ~]# ip link show
...
139: vetha0e043dd@if4: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default
    link/ether 12:78:cc:ea:9b:26 brd ff:ff:ff:ff:ff:ff link-netnsid 3
...
# 检查宿主机主表里的路由规则
[root@node-1 ~]# ip route show
...
10.10.20.53 dev vetha0e043dd scope link
...
# 检查宿主机的自定义路由表
[root@node-1 ~]# ip rule show
...
1024:	from all iif vetha0e043dd lookup 596
...
# 检查宿主机上自定义路由表中的路由规则
[root@node-1 ~]# ip route show table 596
default via 10.10.20.53 dev vetha0e043dd
```

## 测试
```
ip netns add test
# 添加
CNI_PATH=/opt/cni/bin NETCONFPATH=/etc/cni/net.d /usr/local/bin/cnitool add cni0 /var/run/netns/test
# 删除
CNI_PATH=/opt/cni/bin NETCONFPATH=/etc/cni/net.d /usr/local/bin/cnitool del cni0 /var/run/netns/test
ip netns del test
```

## OpenStack辅助测试命令

更新neutron port的辅助ip列表
```
neutron port-update \
    --fixed-ip subnet_id=3088e889-5a39-4e0b-81e6-90fcc71bd2fb,ip_address=192.168.53.48 \
    e2a14f58-f6e2-4f44-b43c-03cc50e14b8b 
```