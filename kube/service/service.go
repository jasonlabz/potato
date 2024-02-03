package service

import (
	"context"
	"fmt"

	"github.com/jasonlabz/potato/kube"
	"github.com/jasonlabz/potato/kube/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func GetServiceList(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (serviceList *corev1.ServiceList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	serviceList, err = kube.GetKubeClient().CoreV1().Services(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetServiceInfo(ctx context.Context, namespace string, serviceName string) (serviceInfo *corev1.Service, err error) {
	getOptions := metav1.GetOptions{}
	serviceInfo, err = kube.GetKubeClient().CoreV1().Services(namespace).Get(ctx, serviceName, getOptions)
	return
}

func DelServiceInfo(ctx context.Context, namespace string, serviceName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().CoreV1().Services(namespace).Delete(ctx, serviceName, delOptions)
	return
}

func WatchService(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().CoreV1().Services(namespace).
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreateServiceRequest struct {
	Namespace                     string                               `json:"namespace"`
	ServiceName                   string                               `json:"service_name"`
	ServiceLabels                 map[string]string                    `json:"service_labels"`
	Annotations                   map[string]string                    `json:"annotations"`
	Selector                      map[string]string                    `json:"selector"`
	Type                          corev1.ServiceType                   `json:"type"`
	ClusterIP                     string                               `json:"cluster_ip"`
	ClusterIPs                    []string                             `json:"cluster_ips"`
	Ports                         []corev1.ServicePort                 `json:"ports,omitempty"`
	ExternalIPs                   []string                             `json:"externalIPs,omitempty" `
	SessionAffinity               corev1.ServiceAffinity               `json:"sessionAffinity,omitempty"`
	LoadBalancerSourceRanges      []string                             `json:"loadBalancerSourceRanges,omitempty" `
	ExternalName                  string                               `json:"externalName,omitempty"`
	ExternalTrafficPolicy         corev1.ServiceExternalTrafficPolicy  `json:"externalTrafficPolicy,omitempty"`
	HealthCheckNodePort           int32                                `json:"healthCheckNodePort,omitempty"`
	PublishNotReadyAddresses      bool                                 `json:"publishNotReadyAddresses,omitempty"`
	SessionAffinityConfig         *corev1.SessionAffinityConfig        `json:"sessionAffinityConfig,omitempty"`
	IPFamilies                    []corev1.IPFamily                    `json:"ipFamilies,omitempty"`
	IPFamilyPolicy                *corev1.IPFamilyPolicy               `json:"ipFamilyPolicy,omitempty"`
	AllocateLoadBalancerNodePorts *bool                                `json:"allocateLoadBalancerNodePorts,omitempty"`
	LoadBalancerClass             *string                              `json:"loadBalancerClass,omitempty"`
	InternalTrafficPolicy         *corev1.ServiceInternalTrafficPolicy `json:"internal_traffic_policy"`
}

func (c *CreateServiceRequest) checkParameters() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if c.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if c.Type == "" {
		return fmt.Errorf("service type is required")
	}

	if c.ExternalTrafficPolicy == "" {
		c.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
	}

	if len(c.ServiceLabels) == 0 {
		c.ServiceLabels = map[string]string{
			"app": c.ServiceName,
		}
	}
	if len(c.Selector) == 0 {
		c.Selector = map[string]string{
			"app": c.ServiceName,
		}
	}

	return nil
}

// CreateService 创建service
/**
apiVersion: v1 	# 指定api版本，此值必须在kubectl api-versions中
kind: Service 	# 指定创建资源的角色/类型
metadata: 	# 资源的元数据/属性
  name: demo 	# 资源的名字，在同一个namespace中必须唯一
  namespace: default 	# 部署在哪个namespace中。不指定时默认为default命名空间
  labels: 		# 设定资源的标签
  - app: demo
  annotations:  # 自定义注解属性列表
  - name: string
spec: 	# 资源规范字段
  type: ClusterIP 	# service的类型，指定service的访问方式，默认ClusterIP。
      # ClusterIP类型：虚拟的服务ip地址，用于k8s集群内部的pod访问，在Node上kube-porxy通过设置的iptables规则进行转发
      # NodePort类型：使用宿主机端口，能够访问各个Node的外部客户端通过Node的IP和端口就能访问服务器
      # LoadBalancer类型：使用外部负载均衡器完成到服务器的负载分发，需要在spec.status.loadBalancer字段指定外部负载均衡服务器的IP，并同时定义nodePort和clusterIP用于公有云环境。
  clusterIP: string		#虚拟服务IP地址，当type=ClusterIP时，如不指定，则系统会自动进行分配，也可以手动指定。当type=loadBalancer，需要指定
  sessionAffinity: string	#是否支持session，可选值为ClietIP，默认值为空。ClientIP表示将同一个客户端(根据客户端IP地址决定)的访问请求都转发到同一个后端Pod
  ports:
    - port: 8080 	# 服务监听的端口号
      targetPort: 8080 	# 容器暴露的端口
      nodePort: int		# 当type=NodePort时，指定映射到物理机的端口号
      protocol: TCP 	# 端口协议，支持TCP或UDP，默认TCP
      name: http 	# 端口名称
  selector: 	# 选择器。选择具有指定label标签的pod作为管理范围
    app: demo
status:	# 当type=LoadBalancer时，设置外部负载均衡的地址，用于公有云环境
  loadBalancer:	# 外部负载均衡器
    ingress:
      ip: string	# 外部负载均衡器的IP地址
      hostname: string	# 外部负载均衡器的主机名

tips: port和nodePort都是service的端口，前者暴露给k8s集群内部服务访问，后者暴露给k8s集群外部流量访问。
从上两个端口过来的数据都需要经过反向代理kube-proxy，流入后端pod的targetPort上，最后到达pod内的容器。
NodePort类型的service可供外部集群访问是因为service监听了宿主机上的端口，即监听了（所有节点）nodePort，
该端口的请求会发送给service，service再经由负载均衡转发给Endpoints的节点。
*/
func CreateService(ctx context.Context, request CreateServiceRequest) (serviceInfo *corev1.Service, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	serviceSrc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.ServiceName,
			Namespace:   request.Namespace,
			Annotations: request.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:                          request.Type,
			Ports:                         request.Ports,
			Selector:                      request.Selector,
			ClusterIP:                     request.ClusterIP,
			ClusterIPs:                    request.ClusterIPs,
			ExternalIPs:                   request.ExternalIPs,
			ExternalName:                  request.ExternalName,
			ExternalTrafficPolicy:         request.ExternalTrafficPolicy,
			LoadBalancerSourceRanges:      request.LoadBalancerSourceRanges,
			HealthCheckNodePort:           request.HealthCheckNodePort,
			PublishNotReadyAddresses:      request.PublishNotReadyAddresses,
			SessionAffinityConfig:         request.SessionAffinityConfig,
			IPFamilies:                    request.IPFamilies,
			IPFamilyPolicy:                request.IPFamilyPolicy,
			AllocateLoadBalancerNodePorts: request.AllocateLoadBalancerNodePorts,
			LoadBalancerClass:             request.LoadBalancerClass,
			InternalTrafficPolicy:         request.InternalTrafficPolicy,
		},
	}
	for labelKey, labelVal := range request.ServiceLabels {
		serviceSrc.ObjectMeta.Labels[labelKey] = labelVal
	}
	options := metav1.CreateOptions{}
	//创建service
	serviceInfo, err = kube.GetKubeClient().CoreV1().Services(request.Namespace).Create(ctx, serviceSrc, options)
	if err != nil {
		return nil, err
	}
	return serviceInfo, err
}
