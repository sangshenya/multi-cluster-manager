# 多集群管理模块设计

最后修改时间：2021/2/21

## 整体设计

![alt](../../img/00-cluster.png)

多集群管理模块将在管理集群中创建对应业务集群的 Cluster 资源映射，该资源包含业务集群名称、状态。

多集群管理模块允许用户在 proxy 中开启自定义插件返回自定义信息，例如监控业务集群的资源使用情况、组件健康状态等信息。

## 设计详情

### Cluster CRD Example

```yaml
apiVersion: stellaris.harmonycloud.cn/v1alpha1
kind: Cluster
metadata:
  name: <cluster name>
spec:
  addons:
    - type: in-tree
      name: <in-tree plugin name>
      configuration: <configuration object>
    - type: out-tree
      name: <out-tree plugin name>
      configuration: <configuration object>
...
status:
  addons:
    - name: <plugin name>
      info: <plugin podInfo object>
      configInfo: <plugin configInfo object>
  conditions:
    - timestamp: "2021-11-02T08:51:39Z"
      message: Apiserver cannot provider service in this cluster
      reason: ApiserverIsDown
      type: KubernetesUnavailable
  lastReceiveHeartBeatTimestamp: <timestamp>
  lastUpdateTimestamp: <timestamp>
  healthy: true/false
  status: online/offline/initializing
```

### 集群纳管

纳管集群：
* 手动再目标集群中部署proxy，并填写管理集群 core 组件地址和管理集群 core 的注册token作为 proxy 启动参数，proxy 与 core 建立连接后，由 core 在管理集群创建 Cluster 对象完成纳管；
  ![alt](../../img/02-manual-create-proxy.png)

### 插件设计

proxy 通过配置开启插件，每一个心跳周期内，proxy 将检查插件返回的数据并增量进行更新。

插件配置设计如下：

```yaml
addons:
  - name: etcd
    type: in-tree
    configuration:
      <plugin configuration object>
  - name: apiserver
    type: in-tree
    configuration:
      <plugin configuration object>
  - name: external-plugin
    type: out-tree
    configuration: 
      http:
      - url: <http url>
```

#### in-tree kube-apiserver-healthy addons

kube-apiserver-healthy 插件将根据配置检测集群 apiserver 组件信息，其配置示例如下：

```yaml
- name: kube-apiserver-healthy
  configurations:
    selector:
    - namespace: kube-system
      labels:
        component: kube-apiserver
    - namespace: kube-system
      include: kube-apiserver
    static:
    - endpoint: https://binaray-apiserver:6443
```

`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`static` 字段将指定 apiserver 实例的地址进行健康检查，通常用于二进制部署的 apiserver 实例。

kube-apiserver-healthy 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: kube-apiserver-healthy
      info:
        - type: pod
          address: https://pod-apiserver:6443
          targetRef:
            name: <pod name>
            namespace: <pod namespace>
          status: ready|notready
        - type: static
          address: https://binaray-apiserver:6443
          status: ready|notready
```
`info` 字段是插件的基础信息

`type` 字段描述该 apiserver 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

#### in-tree kube-controller-manager-healthy addons

kube-controller-manager-healthy 插件将根据配置检测集群 controller-manager 组件信息，其配置示例如下：

```yaml
- name: kube-controller-manager-healthy
  configurations:
    selector:
    - namespace: kube-system
      labels:
        component: kube-controller-manager
    - namespace: kube-system
      include: kube-controller-manager
    static:
    - endpoint: https://binaray-controller-manager:10257
```

`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`static` 字段将指定 controller-manager 实例的地址进行健康检查，通常用于二进制部署的 controller-manager 实例。

kube-controller-manager-healthy 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: kube-controller-manager-healthy
      info:
        - type: pod
          address: https://pod-controller-manager:10257
          targetRef:
            name: <pod name>
            namespace: <pod namespace>
          status: ready|notready
        - type: static
          address: https://binaray-controller-manager:10257
          status: ready|notready
```
`info` 字段是插件的基础信息

`type` 字段描述该 controller-manager 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

#### in-tree kube-scheduler-healthy addons

kube-scheduler-healthy 插件将根据配置检测集群 scheduler 组件信息，其配置示例如下：

```yaml
- name: kube-scheduler-healthy
  configurations:
    selector:
    - namespace: kube-system
      labels:
        component: kube-scheduler
    - namespace: kube-system
      include: kube-scheduler
    static:
    - endpoint: https://binaray-scheduler:10259
```

`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`static` 字段将指定 scheduler 实例的地址进行健康检查，通常用于二进制部署的 scheduler 实例。

kube-scheduler-healthy 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: kube-scheduler-healthy
      info:
        - type: pod
          address: https://pod-scheduler:10259
          targetRef:
            name: <pod name>
            namespace: <pod namespace>
          status: ready|notready
        - type: static
          address: https://binaray-scheduler:10259
          status: ready|notready
```
`info` 字段是插件的基础信息

`type` 字段描述该 scheduler 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

#### in-tree kube-etcd-healthy addons

kube-etcd-healthy 插件将根据配置检测集群 etcd 组件信息，其配置示例如下：

```yaml
- name: kube-etcd-healthy
  configurations:
    selector:
    - namespace: kube-system
      labels:
        component: kube-etcd
    - namespace: kube-system
      include: kube-etcd
    static:
    - endpoint: https://binaray-etcd:2381
```

`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`static` 字段将指定 etcd 实例的地址进行健康检查，通常用于二进制部署的 etcd 实例。

kube-etcd-healthy 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: kube-etcd-healthy
      info:
        - type: pod
          address: https://pod-etcd:2381
          targetRef:
            name: <pod name>
            namespace: <pod namespace>
          status: ready|notready
        - type: static
          address: https://binaray-etcd:2381
          status: ready|notready   
```
`configurations` 字段是插件的配置信息

`info` 字段是插件的基础信息

`type` 字段描述该 etcd 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

#### in-tree coredns addons

coredns 插件将根据配置检测集群 coredns 组件健康状态和配置信息，其配置示例如下：

```yaml
- name: coredns
  configurations:
    selector:
    - namespace: kube-system
      labels:
        k8s-app: kube-dns
    - namespace: kube-system
      include: coredns
    configData:
      configType: configMap
      selector: 
        - namespace: kube-system
          include: coredns
      dataKey: Corefile
      update:
        cacheTime: 60
```
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`configData` 字段表示从数据卷获取配置的相关信息，`configType`表示数据卷类型，`selector`表示数据卷的从目标集群对应命名空间选择符合规则的配置文件，`dataKey`当`configType`为`configMap`时必需，用于从cm中获取实际的配置信息。

`update` 字段表示需要修改配置的信息，以上的示例中表示修改缓存时间为60s，该字段的类型是map[string]interface，字典类型，key是string类型，key的取值范围和下放`configInfo`字段中的`data`中的子字段一致

coredns 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: coredns
      info:
        - type: pod
          address: https://pod-coredns:13145
          targetRef:
            name: <pod name>
            namespace: <pod namespace>
          status: ready|notready
      configInfo:
        data:
          enableErrorLogging: true or false
          cacheTime: 60
          hosts:
            - domain: "."
              resolution:
                - "/etc/resolv.conf"
            - domain: "www.baidu.com"
              resolution:
                - "114.114.114.114"
          forward:
            - domain: "."
              resolution:
                - "/etc/resolv.conf"
            - domain: "www.baidu.com"
              resolution:
                - "114.114.114.114"
          message: <error info> | "success"
```
`info` 字段是插件的基础信息

`configInfo` 字段是插件的配置信息，当配置中设置`configType`字段时有值

`type` 字段描述该 coredns 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

`message` 字段表示获取成功或者获取失败的错误信息

`data` 字段表示获取到的配置数据，获取失败时为空

`enableErrorLogging` 字段表示coredns是否启用错误日志

`cacheTime` 字段表示coredns的缓存时间

`hosts` 字段表示coredns单独配置每一条域名到IP的解析

`forward` 字段表示coredns配置特定域名解析的DNS服务器地址

`domain` 字段表示域名，"."表示所以域名

`resolution` 字段表示解析服务器地址


#### in-tree calico addons

cni 插件将根据配置检查集群中 calico 组件的健康状态，其配置如下：

```yaml
- name: calico
  configurations:
    selector:
    - namespace: kube-system
      include: calico-kube-controllers
    - namespace: kube-system
      labels:
        k8s-app: calico-kube-controllers
``` 
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

calico 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: calico
      info:
      - type: pod
        address: https://pod-calico:10259
        targetRef:
          name: <pod name>
          namespace: <pod namespace>
        status: ready|notready
      - type: static
        address: https://binaray-calico:10259
        status: ready|notready
```

`type` 字段描述该实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

#### in-tree logging addons

logging 插件将根据配置检查集群中 logging监控组件的健康状态以及es的配置信息，其配置如下：

```yaml
- name: logging
  configurations:
    selector:
    - namespace: logging
      include: log-pilot
    - namespace: logging
      include: elasticsearch-master
    configData:
      configType: env
      selector:
        - namespace: logging
          include: elasticsearch-master
      keyList:
        - ELASTIC_PASSWORD
        - TCPPort
        - ClusterName
        - BoxType
      update:
        elasticPassword: Hc@Cloud02
``` 
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`configData` 字段表示从数据卷获取配置的相关信息，`configType`表示数据卷类型，`selector`表示数据卷的从目标集群对应命名空间选择符合规则的配置文件，`dataKey`当`configType`为`configMap`时必需，用于从cm中获取实际的配置信息。

`update` 字段表示需要修改配置的信息，以上的示例中表示修改es的密码为`Hc@Cloud02`，该字段的类型是map[string]interface，字典类型，key是string类型，key的取值范围和下方`configInfo`字段中的`data`中的子字段一致

logging 插件返回的数据示例如下：

```yaml
status:
  addons:
  - name: logging
    info:
    - type: pod
      address: https://pod-es:9200
      targetRef:
        name: <pod name>
        namespace: <pod namespace>
      status: ready|notready
    configInfo:
      data:
        elasticUsername: elastic
        elasticPassword: Hc@Cloud01
        elasticPort: 9200
        elasticClusterName: kubernetes-logging
      message: <error info> | "success"
```

`type` 字段描述该实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

`configInfo` 字段当配置中设置`configType`字段时有值

`message` 字段表示获取成功或者获取失败的错误信息

`data` 字段表示获取到的配置数据，获取失败时为空

`elasticUsername`字段表示获取到的es可视化的登录账号

`elasticPassword`字段表示获取到的es可视化的登录密码

`elasticPort`字段表示获取到的es端口号

`elasticClusterName`字段表示获取到的es集群名称

#### in-tree monitoring addons

monitoring 插件将根据配置检查集群中 Prometheus 监控组件的健康状态以及配置信息，其配置如下：

```yaml
- name: monitoring
  configurations:
    selector:
    - namespace: kubesphere-monitoring-system
      include: prometheus-k8s
    configData:
      configType: prometheus
      selector:
        - namespace: kubesphere-monitoring-system
          include: k8s
      update:
        retention: 7d
``` 
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`configData` 字段表示从数据卷获取配置的相关信息，`configType`表示数据卷类型，`selector`表示数据卷的从目标集群对应命名空间选择符合规则的配置文件。

`update` 字段表示需要修改配置的信息，以上的示例中表示数据保留时间为7d，该字段的类型是map[string]interface，字典类型，key是string类型，key的取值范围和下方`configInfo`字段中的`data`中的子字段一致

logging 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: monitoring
      info:
      - type: pod
        address: https://pod-prometheus
        targetRef:
          name: <pod name>
          namespace: <pod namespace>
        status: ready|notready
      configInfo:
        data:
          retention: 7d
          scrapeInterval: 1m
          version: v2.26.0
          query:
            maxConcurrency: 1000
        message: <error info> | "success"
```

`type` 字段描述该实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康

`configInfo` 字段当配置中设置`configType`字段时有值

`message` 字段表示获取成功或者获取失败的错误信息

`data` 字段表示获取到的配置数据，获取失败时为空

`retention`字段表示prometheus数据保留时长

`scrapeInterval`字段表示检查间隔时间

`version`字段表示部署的prometheus版本

`query`字段表示启动Prometheus时的查询命令行标志。

#### in-tree problem-isolation addons

problem-isolation 插件将根据配置检查集群中 problem-isolation 组件的健康状态，其配置如下：

```yaml
- name: problem-isolation
  configurations:
    selector:
    - namespace: caas-system
      include: problem-isolation
``` 
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

problem-isolation 插件返回的数据示例如下：

```yaml
status:
  addons:
    - name: problem-isolation
      info:
      - type: pod
        address: <pod ip>
        targetRef:
          name: <pod name>
          namespace: <pod namespace>
        status: ready|notready
```

`type` 字段描述该实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康
