package configmap

import (
	"context"
	"fmt"
	"github.com/jasonlabz/potato/kube"
	"github.com/jasonlabz/potato/kube/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func GetConfigmapList(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (configmapList *corev1.ConfigMapList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	configmapList, err = kube.GetKubeClient().CoreV1().ConfigMaps(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetConfigmapInfo(ctx context.Context, namespace string, configmapName string) (configmapInfo *corev1.ConfigMap, err error) {
	getOptions := metav1.GetOptions{}
	configmapInfo, err = kube.GetKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, configmapName, getOptions)
	return
}

func DelConfigmapInfo(ctx context.Context, namespace string, configmapName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().CoreV1().ConfigMaps(namespace).Delete(ctx, configmapName, delOptions)
	return
}

func WatchConfigmap(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().CoreV1().ConfigMaps(namespace).
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreateConfigmapRequest struct {
	Namespace       string            `json:"namespace"`
	ConfigmapName   string            `json:"configmap_name"`
	ConfigmapLabels map[string]string `json:"configmap_labels"`
	Annotations     map[string]string `json:"annotations"`
	Data            map[string]string `json:"data"`
	BinaryData      map[string][]byte `json:"binary_data"`
}

func (c *CreateConfigmapRequest) checkParameters() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if c.ConfigmapName == "" {
		return fmt.Errorf("configmap name is required")
	}

	if len(c.ConfigmapLabels) == 0 {
		c.ConfigmapLabels = map[string]string{
			"configmap": c.ConfigmapName,
		}
	}

	return nil
}

func CreateConfigmap(ctx context.Context, request CreateConfigmapRequest) (configmapInfo *corev1.ConfigMap, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	configmapSrc := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.ConfigmapName,
			Namespace:   request.Namespace,
			Annotations: request.Annotations,
		},
		Data:       request.Data,
		BinaryData: request.BinaryData,
	}
	for labelKey, labelVal := range request.ConfigmapLabels {
		configmapSrc.ObjectMeta.Labels[labelKey] = labelVal
	}
	options := metav1.CreateOptions{}
	//创建configmap
	configmapInfo, err = kube.GetKubeClient().CoreV1().ConfigMaps(request.Namespace).Create(ctx, configmapSrc, options)
	if err != nil {
		return nil, err
	}
	return configmapInfo, err
}
