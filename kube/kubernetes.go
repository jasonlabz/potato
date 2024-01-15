package kube

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

const kubeConfigFilePath = ".kube/config"

type K8sConfig struct {
}

func NewK8sConfig() *K8sConfig {
	return &K8sConfig{}
}

// K8sRestConfig 读取kubeconfig 配置文件
func (k *K8sConfig) K8sRestConfig() *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)

	if err != nil {
		log.Fatal(err)
	}

	return config
}

// InitClient 初始化 clientSet
func (k *K8sConfig) InitClient() *kubernetes.Clientset {
	c, err := kubernetes.NewForConfig(k.K8sRestConfig())

	if err != nil {
		log.Fatal(err)
	}

	return c
}

// InitDynamicClient 初始化 dynamicClient
func (k *K8sConfig) InitDynamicClient() dynamic.Interface {
	c, err := dynamic.NewForConfig(k.K8sRestConfig())

	if err != nil {
		log.Fatal(err)
	}

	return c
}

// InitDiscoveryClient 初始化 DiscoveryClient
func (k *K8sConfig) InitDiscoveryClient() *discovery.DiscoveryClient {
	return discovery.NewDiscoveryClient(k.InitClient().RESTClient())
}
