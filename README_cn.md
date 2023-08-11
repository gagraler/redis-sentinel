# redis-sentinel
基于Kubernetes Operator模式实现Redis Sentinel

## 描述
// TODO: 关于项目和使用概述的深入段落

## 入门
需要一个 Kubernetes 集群来运行, 可以使用 [Kind](https://sigs.k8s.io/kind) 获取本地集群进行测试，或针对远程集群运行。

**注意：** 控制器将自动使用 kubeConfig 文件中的当前上下文（即`kubectl cluster-info`显示的任何集群）。

### 在集群上运行
1. 安装自定义资源实例：

````shell
kubectl apply -f config/samples/
````

1. 构建镜像并将其推送到`IMG`指定的位置：

````shell
make docker-build docker-push IMG=<some-registry>/redis-sentinel-cluster:tag
````

1. 使用“IMG”指定的镜像将控制器部署到集群：

````shell
make deploy IMG=<some-registry>/redis-sentinel-cluster:tag
````

### Uninstall CRD
从集群中删除 CRD

````shell
make uninstall
````

### Undeploy controller
从集群中取消部署控制器：

````shell
make undeploy
````

## 贡献
// TODO: 添加有关希望其他人如何为该项目做出贡献的详细信息

### 如何运行的
该项目旨在遵循 Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)。

使用[Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)，
提供了一个协调功能，负责同步资源，直到集群达到所需的状态

### 测试
1. 将 CRD 安装到集群中：

````shell
make install
````

1. 运行控制器（这将在前台运行，因此如果想让它保持运行，请切换到新终端）：

````shell
make run
````

**注意：** 还可以通过运行以下命令一步运行此命令：`make install run`

### 修改API定义
如果正在编辑 API 定义，请使用以下命令生成清单，例如 CR 或 CRD：

````shell
make manifests
````

**注意：** 运行`“make --help`以获取有关所有潜在`make`目标的更多信息

更多信息可以通过 [Kubebuilder 文档](https://book.kubebuilder.io/introduction.html) 找到

## 许可证

版权所有 2023 keington.

根据 Apache 许可证 2.0 版（“许可证”）获得许可；</br>
除非遵守许可证，否则您不得使用此文件。
您可以在以下位置获取许可证副本

     http://www.apache.org/licenses/LICENSE-2.0

除非适用法律要求或书面同意，否则软件根据许可证分发是在“按原样”基础上分发的，不提供任何明示或暗示的保证或条件。</br>
请参阅许可证以了解特定语言的管理权限和许可证下的限制。
