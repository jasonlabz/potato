package pv

import (
	"context"
	"fmt"
	"github.com/jasonlabz/potato/kube/options"
	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/jasonlabz/potato/kube"
)

func GetPVList(ctx context.Context, opts ...options.ListOptionFunc) (pvList *corev1.PersistentVolumeList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	pvList, err = kube.GetKubeClient().CoreV1().PersistentVolumes().List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetPVInfo(ctx context.Context, pvName string) (pvInfo *corev1.PersistentVolume, err error) {
	getOptions := metav1.GetOptions{}
	pvInfo, err = kube.GetKubeClient().CoreV1().PersistentVolumes().Get(ctx, pvName, getOptions)
	return
}

func DelPVInfo(ctx context.Context, pvName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().CoreV1().PersistentVolumes().Delete(ctx, pvName, delOptions)
	return
}

func WatchPV(ctx context.Context, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().CoreV1().PersistentVolumes().
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreatePVRequest struct {
	PvcName                string                              `json:"pv_name"`
	PvcLabels              map[string]string                   `json:"pv_labels"`
	AccessModes            []corev1.PersistentVolumeAccessMode `json:"access_modes"`
	Storage                string                              `json:"storage"`
	StorageClassName       string                              `json:"storage_class_name"`
	PersistentVolumeSource corev1.PersistentVolumeSource       `json:"persistent_volume_source"`
}

func (c *CreatePVRequest) checkParameters() error {
	if c.PvcName == "" {
		return fmt.Errorf("pv name is required")
	}

	if len(c.PvcLabels) == 0 {
		c.PvcLabels = map[string]string{
			"pv": c.PvcName,
		}
	}

	if len(c.AccessModes) == 0 {
		c.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	}

	return nil
}

func CreatePV(ctx context.Context, request CreatePVRequest) (pvInfo *corev1.PersistentVolume, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	pvSrc := &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   request.PvcName,
			Labels: map[string]string{},
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: request.AccessModes,
			Capacity: corev1.ResourceList{
				"storage": resource.MustParse(request.Storage), //设置存储大小
			},
			PersistentVolumeSource: request.PersistentVolumeSource,
		},
	}
	for labelKey, labelVal := range request.PvcLabels {
		pvSrc.ObjectMeta.Labels[labelKey] = labelVal
	}
	//使用存储卷名字
	if len(request.StorageClassName) != 0 {
		pvSrc.Spec.StorageClassName = request.StorageClassName
	}

	options := metav1.CreateOptions{}
	//创建pv
	pvInfo, err = kube.GetKubeClient().CoreV1().PersistentVolumes().Create(ctx, pvSrc, options)
	if err != nil {
		return nil, err
	}
	return pvInfo, err
}
