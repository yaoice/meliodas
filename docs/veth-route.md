[TOC]

## 适用场景

veth路由模式，基于ptp cni插件改造

## 使用方法

1. 将本cni插件的二进制文件放入`CNI_BIN_PATH`。

2. 正确配置cni的配置文件，如下：

  ```bash
    [root@~]# cat /etc/cni/net.d/10-tcnp.conf 
    {
      "name": "cni0",
      "cniVersion": "0.3.1",
      "type": "veth-route",
      "ipam": {
          "type": "tcnp-ipam",
          "openstackConf": {
             "username": "admin",
             "password": "c111f3c44f352e91ce76",
             "project": "admin",
             "domain": "default",
             "authUrl": "http://10.125.22.21:35357/v3"
          },
          "neutronConf": {
             "networks": ["b9535039-d935-4a33-a7cb-2cc1c58ab904"],
             "mode": "mix"
          },
          "routes": [{
              "dst": "0.0.0.0/0"
          }]
      }
    }
  ```

3. 搭配vrouter-route-controller一起使用， vrouter-route-controller负责自动刷新vrouter静态路由表
  
## veth路由模式原理

容器的地址是neutron port的多IP中的IP，必须有vrouter存在；
容器发送出去的包通过VM的网卡出，回来的包需要在vrouter和宿主机设置静态路由

```
(10.10.10.8)vm -> vrouter(10.10.10.1)
           /  \
          /    \
         /      \
       容器      容器
(10.10.10.19)   (10.10.10.20) 
```
虚拟机vm ip为10.10.10.8，默认网关指向的vrouter地址是10.10.10.1

1. 容器与宿主机通过veth对和路由进行通信
```
ip netns add test
CNI_PATH=/opt/cni/bin NETCONFPATH=/etc/cni/net.d /usr/local/bin/cnitool add cni0 /var/run/netns/test

[root@test-tcnp ~]# ip netns exec test ip a
1: lo: <LOOPBACK> mtu 65536 qdisc noop state DOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
3: eth0@if46: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether 72:96:91:d4:58:36 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 10.10.10.19/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::7096:91ff:fed4:5836/64 scope link 
       valid_lft forever preferred_lft forever

[root@test-tcnp ~]# ip netns exec test ip route
default via 169.254.1.1 dev eth0 
169.254.1.1 dev eth0 scope link 
```
eth0与宿主机 id为46的网卡构成veth对，在命名空间里默认网关指向169.254.1.1(宿主机dummy网卡)

2. 宿主机的veth网卡和路由
```
4: dummy0: <BROADCAST,NOARP,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether f2:37:21:fc:93:8d brd ff:ff:ff:ff:ff:ff
    inet 169.254.1.1/32 scope global dummy0
       valid_lft forever preferred_lft forever
    inet6 fe80::f037:21ff:fefc:938d/64 scope link 
       valid_lft forever preferred_lft forever

46: veth7c1b453d@if3: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether 0e:12:32:5b:a7:f9 brd ff:ff:ff:ff:ff:ff link-netnsid 18
    inet6 fe80::c12:32ff:fe5b:a7f9/64 scope link 
       valid_lft forever preferred_lft forever
```
和容器的eth0网卡构成veth对，通过id和@if3可以看出来

宿主机到容器的路由
```
[root@test-tcnp ~]# ip route
default via 10.10.10.1 dev eth0 
10.10.10.0/24 dev eth0 proto kernel scope link src 10.10.10.8 
10.10.10.19 dev veth7c1b453d
```
路由scope采用global(默认不显示)

```
SCOPE := [ host | link | global | NUMBER ]
```
- Global: 可以转发，例如从一个端口收到的包，可以查询global的路由条目，如果目的地址在另外一个网卡，那么该路由条目可以匹配转发的要求，进行路由转发
- Link: scope路由条目是不会转发任何匹配的数据包到其他的硬件网口的，link是在链路上才有效，这个链路是指同一个端口，也就是说接收和发送都是走的同一个端口的时候，这条路由才会生效（也就是说在同一个二层）
- Host: 表示这是一条本地路由，典型的是回环端口，loopback设备使用这种路由条目，该路由条目比link类型的还要严格，约定了都是本机内部的转发，不可能转发到外部
- NUMBER: ipv6专用的路由scope


3. vrouter的路由
```
[root@ ~]# ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
29: qr-6382d00a-11: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether fa:16:3e:08:c0:4d brd ff:ff:ff:ff:ff:ff
    inet 10.10.10.1/24 brd 10.10.10.255 scope global qr-6382d00a-11
       valid_lft forever preferred_lft forever
    inet6 fe80::f816:3eff:fe08:c04d/64 scope link 
       valid_lft forever preferred_lft forever
94: qg-5560d7d3-dc: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default qlen 1000
    link/ether fa:16:3e:35:9a:a6 brd ff:ff:ff:ff:ff:ff
    ......

[root@ ~]# ip route
default via 10.125.224.1 dev qg-5560d7d3-dc 
10.10.10.0/24 dev qr-6382d00a-11 proto kernel scope link src 10.10.10.1 
10.10.10.19 via 10.10.10.8 dev qr-6382d00a-11 
10.125.224.0/26 dev qg-5560d7d3-dc proto kernel scope link src 10.125.224.34 
```
一个vrouter实际上就是一个namespace，告诉vrouter，到容器地址10.10.10.19路由的下一跳是其宿主机地址10.10.10.8; 调用neutron接口可以往vrouter里注入静态路由,
还可以绑定浮动ip直接映射到10.10.10.19(因为这也是neutron port多IP的合法IP)

4. 销毁容器
```
CNI_PATH=/opt/cni/bin NETCONFPATH=/etc/cni/net.d /usr/local/bin/cnitool del cni0 /var/run/netns/test
ip netns del test
```

## 限制

```
The number of routes exceeds the maximum 30.", "type": "RoutesExhausted", "detail": ""}}
```
vrouter的静态路由默认最大是30条
