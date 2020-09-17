[TOC]

## neutron ipam插件

基于[host-local](https://github.com/containernetworking/plugins/tree/master/plugins/ipam/host-local)插件二次开发,
neutron ipam插件从neutron地址池分配ip

## cni sample配置文件

这个例子返回1个ip地址
```json
{
     "ipam": {
        "name": "tcnp-ipam",
        "type": "tcnp-ipam",
        "openstackConf": {
            "username": "admin",
            "password": "c111f3c44f352e91ce76",
            "project": "admin",
            "domain": "default",
            "authUrl": "http://1.1.1.1:35357/v3"
        },
        "neutronConf": {
            "mode": "mix",
            "hostInterface": "eth0",
            "networks": ["782ec9ac-44f9-4318-8c67-a2fed2ccca4f"]
        },
        "routes": [
            { "dst": "0.0.0.0/0" },
            { "dst": "192.168.0.0/16", "gw": "10.10.5.1" }
        ]
     }
}
```

## 命令行测试ipam
```bash
# 添加
$ echo '{"cniVersion": "0.3.1","name": "examplenet","ipam": {"name": "tcnp-ipam","type": "tcnp-ipam","openstackConf": {"username": "admin","password": "c111f3c44f352e91ce76","project": "admin","domain": "default","authUrl": "http://10.125.224.21:35357/v3"},"neutronConf": {"mode": "mix", networks": ["782ec9ac-44f9-4318-8c67-a2fed2ccca4f"]}}}' | CNI_COMMAND=ADD CNI_CONTAINERID=example CNI_NETNS=/dev/null CNI_IFNAME=dummy0 CNI_PATH=. ./tcnp-ipam

# 删除 
$ echo '{"cniVersion": "0.3.1","name": "examplenet","ipam": {"name": "tcnp-ipam","type": "tcnp-ipam","openstackConf": {"username": "admin","password": "c111f3c44f352e91ce76","project": "admin","domain": "default","authUrl": "http://10.125.224.21:35357/v3"},"neutronConf": {"mode": "mix", networks": ["782ec9ac-44f9-4318-8c67-a2fed2ccca4f"]}}}' | CNI_COMMAND=DEL CNI_CONTAINERID=example CNI_NETNS=/dev/null CNI_IFNAME=dummy0 CNI_PATH=. ./tcnp-ipam

# 可以使用以下命令转换
# awk BEGIN{RS=EOF}'{gsub(/\n/,"");print}' /tmp/xxx.json
```

返回结果
```json
{
    "ips": [
        {
            "version": "4",
            "address": "203.0.113.2/24",
            "gateway": "203.0.113.1"
        }
    ],
    "dns": {}
}
```

## 参数配置解释

* `type` (string, required): "tcnp-ipam".
* `routes` (string, optional): 路由信息
* `resolvConf` (string, optional): dns配置
* `neutronConf`, OpenStack Neutron配置
  * `mode` (string, required): 模式，可选值为separate, mix, eni
  * `networks` (array, required, nonempty) neutron网络id
* `openstackConf`, OpenStack Auth配置
  * `username` (string, required): 用户名
  * `password` (string, required): 密码(用scripts/encrypt.go加密的结果)
  * `project` (string, required): 项目名
  * `domain` (string, required): 域(keystone v3)
  * `authUrl` (string, required): 认证url



