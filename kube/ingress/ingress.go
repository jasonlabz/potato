package ingress

import (
	"context"
	"fmt"
	netv1 "k8s.io/api/networking/v1"

	"github.com/jasonlabz/potato/kube"
	"github.com/jasonlabz/potato/kube/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func GetIngressList(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (ingressList *netv1.IngressList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	ingressList, err = kube.GetKubeClient().NetworkingV1().Ingresses(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetIngressInfo(ctx context.Context, namespace string, ingressName string) (ingressInfo *netv1.Ingress, err error) {
	getOptions := metav1.GetOptions{}
	ingressInfo, err = kube.GetKubeClient().NetworkingV1().Ingresses(namespace).Get(ctx, ingressName, getOptions)
	return
}

func DelIngressInfo(ctx context.Context, namespace string, ingressName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().NetworkingV1().Ingresses(namespace).Delete(ctx, ingressName, delOptions)
	return
}

func WatchIngress(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().NetworkingV1().Ingresses(namespace).
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreateIngressRequest struct {
	Namespace        string                `json:"namespace"`
	IngressName      string                `json:"ingress_name"`
	IngressLabels    map[string]string     `json:"ingress_labels"`
	Annotations      map[string]string     `json:"annotations"`
	DefaultBackend   *netv1.IngressBackend `json:"default_backend"`
	IngressClassName *string               `json:"ingress_class_name"`
	TLS              []netv1.IngressTLS    `json:"tls,omitempty"`
	Rules            []netv1.IngressRule   `json:"rules"`
}

func (c *CreateIngressRequest) checkParameters() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if c.IngressName == "" {
		return fmt.Errorf("ingress name is required")
	}

	if len(c.IngressLabels) == 0 {
		c.IngressLabels = map[string]string{
			"app": c.IngressName,
		}
	}

	return nil
}

// CreateIngress 创建ingress
/**
apiVersion: extensions/v1beta1 		# 创建该对象所使用的 Kubernetes API 的版本
kind: Ingress 		# 想要创建的对象的类别，这里为Ingress
metadata:
  name: showdoc		# 资源名字，同一个namespace中必须唯一
  namespace: op 	# 定义资源所在命名空间
  annotations: 		# 自定义注解
    kubernetes.io/ingress.class: nginx    	# 声明使用的ingress控制器
spec:
  rules:
  - host: showdoc.example.cn     # 服务的域名
    http:
      paths:
      - path: /      # 路由路径
        backend:     # 后端Service
          serviceName: showdoc		# 对应Service的名字
          servicePort: 80           # 对应Service的端口

*/
func CreateIngress(ctx context.Context, request CreateIngressRequest) (ingressInfo *netv1.Ingress, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	ingressSrc := &netv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.IngressName,
			Namespace:   request.Namespace,
			Annotations: request.Annotations,
		},
		Spec: netv1.IngressSpec{
			IngressClassName: request.IngressClassName,
			DefaultBackend:   request.DefaultBackend,
			TLS:              request.TLS,
			Rules:            request.Rules,
		},
	}
	for labelKey, labelVal := range request.IngressLabels {
		ingressSrc.ObjectMeta.Labels[labelKey] = labelVal
	}
	options := metav1.CreateOptions{}
	//创建ingress
	ingressInfo, err = kube.GetKubeClient().NetworkingV1().Ingresses(request.Namespace).Create(ctx, ingressSrc, options)
	if err != nil {
		return nil, err
	}
	return ingressInfo, err
}
