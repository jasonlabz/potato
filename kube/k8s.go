package kube

import (
	"k8s.io/client-go/kubernetes"
)

var client *kubernetes.Clientset

func init() {
	client = initClient()
}

func GetKubeClient() *kubernetes.Clientset {
	return client
}

func GetKubeClientApi() *kubernetes.Clientset {
	GetKubeClient().CoreV1()
	return client
}
