package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jasonlabz/potato/utils"
)

var client *kubernetes.Clientset

var (
	clientOnce sync.Once
	clientErr  error
)

// GetKubeClient 首次调用时初始化 clientset；初始化失败时 panic，与原先 init 中的语义一致，
// 只是推迟到首次使用。需要先判断可用性时调用 KubeClientError。
func GetKubeClient() *kubernetes.Clientset {
	if err := KubeClientError(); err != nil {
		panic(fmt.Errorf("kube client init: %w", err))
	}
	return client
}

// KubeClientError 返回 clientset 初始化失败的原因，不会 panic。
func KubeClientError() error {
	clientOnce.Do(func() {
		client, clientErr = initClient()
	})
	return clientErr
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
