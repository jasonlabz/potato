package kube

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jasonlabz/potato/log"
	"github.com/jasonlabz/potato/utils"
)

var client *kubernetes.Clientset

func init() {
	client = initClient()
}

func GetKubeClient() *kubernetes.Clientset {
	return client
}

// k8sRestConfig 读取kubeconfig 配置文件
func k8sRestConfig() *rest.Config {
	kubeConfigFilePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if !utils.IsExist(kubeConfigFilePath) {
		kubeConfigFilePath = ""
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
	if err != nil {
		log.GetLogger().WithError(err).Fatal("load kube config fail")
	}
	return config
}

// initClient 初始化 clientSet
func initClient() *kubernetes.Clientset {
	c, err := kubernetes.NewForConfig(k8sRestConfig())

	if err != nil {
		log.GetLogger().WithError(err).Fatal("init kube clientSet fail")
	}

	return c
}

// initDynamicClient 初始化 dynamicClient
func initDynamicClient() dynamic.Interface {
	c, err := dynamic.NewForConfig(k8sRestConfig())

	if err != nil {
		log.GetLogger().WithError(err).Fatal("init kube dynamicClient fail")
	}

	return c
}

// initDiscoveryClient 初始化 DiscoveryClient
func initDiscoveryClient() *discovery.DiscoveryClient {
	return discovery.NewDiscoveryClient(initClient().RESTClient())
}
