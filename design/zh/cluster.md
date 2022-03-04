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
  apiserver: <apiserver addr>
  secretRef:
    type: kubeconfig # or use token
    name: cluster-238-secret
    namespace: stellaris-system
    field: admin.conf
  addons:
    - type: in-tree
      name: <in-tree plugin name>
      configuration: <configuration object>
    - type: out-tree
      name: <out-tree plugin name>
      url: <out tree plugin url>
...
status:
  addons:
    - name: cluster.skyview.harmonycloud
      info: <info object>
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

纳管集群共分为两种模式：
* 通过在管理集群中创建 Cluster 对象，由 core 向目标集群中部署 proxy 完成纳管；
  ![alt](../../img/01-auto-create-proxy.png)
* 手动再目标集群中部署proxy，并填写管理集群 core 组件地址作为 proxy 启动参数，proxy 与 core 建立连接后，由 core 在管理集群创建 Cluster 对象完成纳管；
  ![alt](../../img/02-manual-create-proxy.png)

#### 自动部署 Proxy

自动部署时，用户在管理集群创建 Cluster 对象，必须填入以下参数：

| 字段                     | 释义                                                       | 示例                 |
| ------------------------ | ---------------------------------------------------------- | -------------------- |
| spec.apiserver           | 管理集群至业务集群 apiserver 可达地址                      | https://1.2.3.4:8443 |
| spec.secretRef.type      | 管理集群中至业务集群访问秘钥类型，可选 token 或 kubeconfig | token                |
| spec.secretRef.name      | 管理集群中对应业务集群的秘钥名称                           | cluster-x-secret     |
| spec.secretRef.namespace | 管理集群中对应业务集群的秘钥所在命名空间                   | stellaris-system     |
| spec.secretRef.field     | 管理集群中对应业务集群的秘钥在 Secret 中对应字段           | admin.conf           |

通过设置 core 启动参数 `--cue-template-config-map` 指定读取的 ConfigMap，core 将读取该 ConfigMap 中 `deploy-proxy.cue` 字段。

其中 CUE 模板如下：
```cuelang
// 上下文对象目前仅包含 context.name (集群名称)
context: {}

// 通过 cluster.spec.configuration 作为入参
parameters: {}

// XX 为不重复的 key 值，value 作为部署到业务集群的资源
outputs.XX: {}
```

默认预置的 template 包含以下参数：

```cuelang
parameters: {
    // 资源名称
    name: *"stellaris-proxy" | string
    // 资源命名空间
    namespace: *"stellaris-system" | string
    // proxy 运行副本数
    replicas: *1 | int
    // proxy 镜像
    image: string
    // proxy 连接 core 使用的地址
    coreAddr: string
    // proxy 在业务集群中使用的 ClusterRole
    clusterRole: string
}
```

自动部署时，用户在管理集群创建 Cluster 对象，可以通过配置 `spec.addons` 决定自动部署的 proxy 需要开启哪些插件。

### 插件设计

proxy 通过配置开启插件，每一个心跳周期内，proxy 将检查插件返回的数据并增量进行更新。

插件配置设计如下：

```yaml
plugins:
  inTree:
  - name: etcd
    configuration:
      <plugin configuration object>
  - name: apiserver
    configuration:
      <plugin configuration object>
  outTree:
  - name: external-plugin
    http:
      url: <http url>
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

`type` 字段描述该 apiserver 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康，通过 apiserver 实例的 /healthz 接口判断

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

`static` 字段将指定 controller-manager 实例的地址进行健康检查，通常用于二进制部署的 apiserver 实例。

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

`type` 字段描述该 controller-manager 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康，通过 controller-manager 实例的 /healthz 接口判断

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

`static` 字段将指定 scheduler 实例的地址进行健康检查，通常用于二进制部署的 apiserver 实例。

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

`type` 字段描述该 scheduler 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康，通过 scheduler 实例的 /healthz 接口判断

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

`type` 字段描述该 etcd 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康，通过 etcd 实例的 /health 接口判断

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
    volumesType: configMap
```
`selector` 字段将从目标集群对应命名空间选择符合规则的 pod，`labels` 代表以标签作为选择的依据，`include` 则表示通过名称模糊匹配搜索对应的 pod。

`volumesType` 字段表示获取到pod之后从数据卷获取配置信息的类型。

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
    volumesInfo:
      data:
        enableErrorLogging: true or false
        cacheTime: 30
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

`type` 字段描述该 coredns 实例的类型是目标集群的 pod 或静态地址

`targetRef` 字段在类型为 pod 时有效，描述该实例在目标集群中的位置

`address` 描述该实例静态地址，pod 将返回其 podIP 作为地址

`status` 字段描述该实例目前是否健康，通过 coredns 实例的 /health 接口判断

`volumesInfo` 字段当配置中设置`volumesType`字段时有值

`message` 字段表示获取成功或者获取失败的错误信息

`data` 字段表示获取到的配置数据，获取失败时为空

`enableErrorLogging` 字段表示coredns是否启用错误日志

`cacheTime` 字段表示coredns的缓存时间

`hosts` 字段表示coredns单独配置每一条域名到IP的解析

`forward` 字段表示coredns配置特定域名解析的DNS服务器地址

`domain` 字段表示域名，"."表示所以域名

`resolution` 字段表示解析服务器地址
