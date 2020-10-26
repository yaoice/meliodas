# 手动实践ipvlan

## 环境

OpenStack虚拟机环境
```
# uname -r
4.4.236-1.el7.elrepo.x86_64
```

## 实验步骤

1. 创建两个测试的network namespace
```
# ip netns add test1
# ip netns add test2
```

2. 创建l2模式的ipvlan虚拟网卡接口 
```
# ip link add test1 link eth0 type ipvlan mode l2
# ip link add test2 link eth0 type ipvlan mode l2
```

3. 把ipvlan虚拟网卡接口放到network namespace中
```
# ip link set test1 netns test1
# ip link set test2 netns test2
# ip netns exec test1 ip link set test1 up
# ip netns exec test2 ip link set test2 up
```

4. 给ipvlan虚拟网卡接口配置ip&路由
```
# ip netns exec test1 ip addr add 192.168.104.211/24 dev test1
# ip netns exec test2 ip addr add 192.168.104.212/24 dev test2
# ip netns exec test1 ip route add default dev test1
# ip netns exec test2 ip route add default dev test2
```

5. 测试连通性
```
#测试到test2命名空间的ipvlan虚拟网卡的连通性
# ip netns exec test1 ping 192.168.104.212
PING 192.168.104.212 (192.168.104.212) 56(84) bytes of data.
64 bytes from 192.168.104.212: icmp_seq=1 ttl=64 time=0.391 ms
64 bytes from 192.168.104.212: icmp_seq=2 ttl=64 time=0.085 ms
^C
--- 192.168.104.212 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 999ms
rtt min/avg/max/mdev = 0.085/0.238/0.391/0.153 ms

#测试到test1命名空间的ipvlan虚拟网卡的连通性
# ip netns exec test2 ping 192.168.104.211
PING 192.168.104.211 (192.168.104.211) 56(84) bytes of data.
64 bytes from 192.168.104.211: icmp_seq=1 ttl=64 time=0.284 ms
64 bytes from 192.168.104.211: icmp_seq=2 ttl=64 time=0.094 ms
^C
--- 192.168.104.211 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1000ms
rtt min/avg/max/mdev = 0.094/0.189/0.284/0.095 ms

#测试到网关的连通性
# ip netns exec test2 ping 192.168.104.254
PING 192.168.104.254 (192.168.104.254) 56(84) bytes of data.
64 bytes from 192.168.104.254: icmp_seq=1 ttl=255 time=6.92 ms
^C
--- 192.168.104.254 ping statistics ---
1 packets transmitted, 1 received, 0% packet loss, time 0ms
rtt min/avg/max/mdev = 6.925/6.925/6.925/0.000 ms

#测试到网关的连通性
# ip netns exec test1 ping 192.168.104.254
PING 192.168.104.254 (192.168.104.254) 56(84) bytes of data.
64 bytes from 192.168.104.254: icmp_seq=1 ttl=255 time=1.32 ms
64 bytes from 192.168.104.254: icmp_seq=2 ttl=255 time=0.939 ms
^C
--- 192.168.104.254 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1001ms
rtt min/avg/max/mdev = 0.939/1.130/1.321/0.191 ms
```

6. 从network namespace移除ipvlan虚拟网卡
```
# ip netns exec test1 ip link delete test1
# ip netns exec test2 ip link delete test2
```

7. 测试ipvlan l3模式的虚拟网卡连通性
```
# ip link add test1 link eth0 type ipvlan mode l3 
```

8. 重复上述3、5、5的步骤
```
# ip netns exec test1 ip link set test1 up
# ip netns exec test1 ip addr add 192.168.104.211/24 dev test1
# ip netns exec test1 ip route add default dev test1

# ip netns exec test1 ping 192.168.104.254
PING 192.168.104.254 (192.168.104.254) 56(84) bytes of data.
64 bytes from 192.168.104.254: icmp_seq=1 ttl=255 time=1.76 ms
64 bytes from 192.168.104.254: icmp_seq=2 ttl=255 time=0.996 ms
^C

# ping 192.168.104.211
PING 192.168.104.211 (192.168.104.211) 56(84) bytes of data.\
From 192.168.104.111 icmp_seq=1 Destination Host Unreachable
From 192.168.104.111 icmp_seq=2 Destination Host Unreachable
From 192.168.104.111 icmp_seq=3 Destination Host Unreachable
From 192.168.104.111 icmp_seq=4 Destination Host Unreachable
```
其它同网段的主机ping不通命名空间的ipvlan虚拟网卡，这是因为如果使用L3模式，
pod会ping不通网关，arp过程都是在父接口完成的，你需要在其宿主机上手动发起arp请求

```
# arping -c 2 -U -I eth0 192.168.104.211
# ping 192.168.104.211
PING 192.168.104.211 (192.168.104.211) 56(84) bytes of data.
64 bytes from 192.168.104.211: icmp_seq=1 ttl=64 time=1.63 ms
64 bytes from 192.168.104.211: icmp_seq=2 ttl=64 time=0.976 ms
^C
```
可以连通一段时间，经测试过一段时间后，arp也会失效
