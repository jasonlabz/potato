# kube

Kubernetes 客户端封装，提供常用 K8s 资源的 CRUD 操作。

## 目录结构

```
kube/
├── client.go       # 客户端初始化（自动加载 kubeconfig）
├── body.go         # 数据结构定义
├── configmap/      # ConfigMap 操作
├── deployment/     # Deployment 操作
├── ingress/        # Ingress 操作
├── pv/             # PersistentVolume 操作
├── pvc/            # PersistentVolumeClaim 操作
├── service/        # Service 操作
├── storageclass/   # StorageClass 操作
└── options/        # 通用选项
```

## 使用示例

```go
import "github.com/jasonlabz/potato/kube"

// 获取 K8s 客户端（自动从 ~/.kube/config 或集群内配置加载）
client := kube.GetKubeClient()
```

## 支持的资源

- Deployment - 创建/更新/删除/列表
- Service - 创建/更新/删除/列表
- ConfigMap - 创建/更新/删除/列表
- Ingress - 创建/更新/删除/列表
- PersistentVolume / PersistentVolumeClaim - 存储资源管理
- StorageClass - 存储类管理

## 依赖

- `k8s.io/client-go`
- `k8s.io/api`
- `k8s.io/apimachinery`
