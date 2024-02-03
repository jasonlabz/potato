package storageclass

import (
	"context"
	"fmt"

	"github.com/jasonlabz/potato/kube"
	"github.com/jasonlabz/potato/kube/options"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func GetStorageClassList(ctx context.Context, opts ...options.ListOptionFunc) (storageList *storagev1.StorageClassList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	storageList, err = kube.GetKubeClient().StorageV1().StorageClasses().List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetStorageClassInfo(ctx context.Context, storageName string) (storageInfo *storagev1.StorageClass, err error) {
	getOptions := metav1.GetOptions{}
	storageInfo, err = kube.GetKubeClient().StorageV1().StorageClasses().Get(ctx, storageName, getOptions)
	return
}

func DelStorageClassInfo(ctx context.Context, storageName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().StorageV1().StorageClasses().Delete(ctx, storageName, delOptions)
	return
}

func WatchStorageClass(ctx context.Context, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().StorageV1().StorageClasses().
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreateStorageClassRequest struct {
	StorageClassName              string                                `json:"storage_class_name"`
	StorageClassLabels            map[string]string                     `json:"storage_class_labels"`
	AccessModes                   []corev1.PersistentVolumeAccessMode   `json:"access_modes"`
	Storage                       string                                `json:"storage"`
	PersistentVolumeReclaimPolicy *corev1.PersistentVolumeReclaimPolicy `json:"persistent_volume_reclaim_policy"`
}

func (c *CreateStorageClassRequest) checkParameters() error {
	if c.StorageClassName == "" {
		return fmt.Errorf("storageClass name is required")
	}

	if len(c.StorageClassLabels) == 0 {
		c.StorageClassLabels = map[string]string{
			"storage": c.StorageClassName,
		}
	}

	if len(c.AccessModes) == 0 {
		c.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	}

	return nil
}

func CreateStorageClass(ctx context.Context, request CreateStorageClassRequest) (storageInfo *storagev1.StorageClass, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	storageSrc := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   request.StorageClassName,
			Labels: map[string]string{},
		},
		ReclaimPolicy: request.PersistentVolumeReclaimPolicy,
	}
	for labelKey, labelVal := range request.StorageClassLabels {
		storageSrc.ObjectMeta.Labels[labelKey] = labelVal
	}

	options := metav1.CreateOptions{}
	//创建storage
	storageInfo, err = kube.GetKubeClient().StorageV1().StorageClasses().Create(ctx, storageSrc, options)
	if err != nil {
		return nil, err
	}
	return storageInfo, err
}
