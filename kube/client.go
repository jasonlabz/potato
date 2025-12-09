package kube

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jasonlabz/potato/utils"
)

var client *kubernetes.Clientset

func init() {
	var err error
	client, err = initClient()
	if err != nil {
		panic(err)
	}
}

func GetKubeClient() *kubernetes.Clientset {
	return client
}

// k8sRestConfig 读取kubeconfig 配置文件
func k8sRestConfig() (*rest.Config, error) {
	kubeConfigFilePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if !utils.IsExist(kubeConfigFilePath) {
		kubeConfigFilePath = ""
	}
	return clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
}

// initClient 初始化 clientSet
func initClient() (*kubernetes.Clientset, error) {
	config, err := k8sRestConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// initDynamicClient 初始化 dynamicClient
func initDynamicClient() (dynamic.Interface, error) {
	config, err := k8sRestConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

// initDiscoveryClient 初始化 DiscoveryClient
func initDiscoveryClient() (*discovery.DiscoveryClient, error) {
	clientset, err := initClient()
	if err != nil {
		return nil, err
	}
	return discovery.NewDiscoveryClient(clientset.RESTClient()), nil
}
