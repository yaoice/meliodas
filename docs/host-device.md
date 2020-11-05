## 适用场景

弹性网卡直通模式, 低延迟，性能好

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
              "type": "host-device",
              "ipam": {
                  "type": "tcnp-ipam",
                  "openstackConf": {
                     "username": "admin",
                     "password": "b974a1991171",
                     "project": "admin",
                     "domain": "default",
                     "authUrl": "http://192.168.51.250:35357/v3"
                  },
                  "neutronConf": {
                     "networks": ["73fe8ad8-eda8-4ee0-9d10-ffca11752362"],
                     "mode": "eni"
                  },
                  "routes": [{
                      "dst": "0.0.0.0/0"
                  }]
              }
          },
          {
              "type": "veth-host",
              "serviceCidr": "10.96.0.0/12",
              "hostInterface": "eth0",
              "containerInterface": "veth0",
              "ipMasq": true
          }
      ]
    }
  ```
  
## 网卡直通原理

默认docker实例被创建出来后，ip netns(从/var/run/netns读取)无法看到容器实例对应的namespace.

查找容器的主进程ID
```
# docker inspect --format '{{.State.Pid}}' <docker实例名字或ID>
# docker inspect --format '{{.State.Pid}}' ae06166543d7
```

创建/var/run/netns 目录以及符号连接 
```
# mkdir /var/run/netns
# ln -s /proc/<容器的主进程ID>/ns/net /var/run/netns/<docker实例名字或ID>
# ln -s /proc/21944/ns/net /var/run/netns/ae06166543d7
# ip netns
ae06166543d7 (id: 1)
```

进入namespace查看
```
# ip netns exec ae06166543d7 ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
3: veth0@if26: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether a2:f9:57:16:c2:6d brd ff:ff:ff:ff:ff:ff link-netnsid 0
25: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether fa:16:3e:4d:76:00 brd ff:ff:ff:ff:ff:ff
    inet 192.168.52.30/24 brd 192.168.52.255 scope global eth0
       valid_lft forever preferred_lft forever
```

把宿主机网卡设备添加到namespace
```
# ip link set dev23 netns ae06166543d7
# ip netns exec ae06166543d7 ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
3: veth0@if26: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether a2:f9:57:16:c2:6d brd ff:ff:ff:ff:ff:ff link-netnsid 0
23: dev23: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN group default qlen 1000
    link/ether fa:16:3e:95:e5:b5 brd ff:ff:ff:ff:ff:ff
25: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether fa:16:3e:4d:76:00 brd ff:ff:ff:ff:ff:ff
    inet 192.168.52.30/24 brd 192.168.52.255 scope global eth0
       valid_lft forever preferred_lft forever
```

从namespace中移除宿主机网卡设备
```
# ip netns exec ae06166543d7 ip link set dev23 netns 1
# ip netns exec ae06166543d7 ip a
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
3: veth0@if26: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether a2:f9:57:16:c2:6d brd ff:ff:ff:ff:ff:ff link-netnsid 0
25: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether fa:16:3e:4d:76:00 brd ff:ff:ff:ff:ff:ff
    inet 192.168.52.30/24 brd 192.168.52.255 scope global eth0
       valid_lft forever preferred_lft forever
```

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

## 小技巧
nsenter命令是一个可以在指定进程的命令空间下运行指定程序的命令, 可以进入该容器的网络命名空间
```
# sudo nsenter --target 21944 --uts --ipc --net --pid /bin/sh
/ # 
```

## OpenStack辅助测试命令

打印nova instance的网卡列表
```
for id in `nova list |grep -E "192.168.52.33|192.168.52.15" \
    | awk '{print $2}'`;do nova interface-list $id \
    |grep -v -E "192.168.52.15|192.168.52.33";done
```

