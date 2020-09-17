[TOC]

## 适用场景

跟OpenStack融合打通下，根据虚拟机网卡辅助IP/直通网卡的空闲情况来调度

## 使用方法

1. 编译meliodas-scheduler镜像

    ```
    make local_image
    ```

2. 修改kube-scheduler manifest配置

    ```
      - command:
        - kube-scheduler
        - --config=/etc/kubernetes/kubescheduler.yaml
      image: xxx.com/scheduler-plugins:v1
    ```

3. 创建/etc/kubernetes/kubescheduler.yaml配置文件
 
    ```
    apiVersion: kubescheduler.config.k8s.io/v1alpha2
    kind: KubeSchedulerConfiguration
    leaderElection:
      leaderElect: true
    clientConnection:
      kubeconfig: "/etc/kubernetes/scheduler.conf"
    profiles:
    - schedulerName: default-scheduler
      plugins:
        queueSort:
          enabled:
            - name: Scheduling
          disabled:
            - name: "*"
        filter:
          enabled:
            - name: Scheduling
        preScore:
          enabled:
            - name: Scheduling
          disabled:
            - name: "*"
        score:
          enabled:
            - name: Scheduling
          disabled:
            - name: "*"
    # optional plugin configs
      pluginConfig: 
      - name: Scheduling
        args:
          userName: "admin"
          passWord: "b974a1991171"
          project: "admin"
          domain: "default"
          authUrl: "http://192.168.101.250:35357/v3"
    ```
根据环境填写OpenStack对应配置信息，passWord通过scripts/encrypt.go文件生成

