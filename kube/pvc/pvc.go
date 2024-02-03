package pvc

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/jasonlabz/potato/kube"
	"github.com/jasonlabz/potato/kube/options"
)

func GetPVCList(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (pvcList *corev1.PersistentVolumeClaimList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	pvcList, err = kube.GetKubeClient().CoreV1().PersistentVolumeClaims(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetPVCInfo(ctx context.Context, namespace string, pvcName string) (pvcInfo *corev1.PersistentVolumeClaim, err error) {
	getOptions := metav1.GetOptions{}
	pvcInfo, err = kube.GetKubeClient().CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, getOptions)
	return
}

func DelPVCInfo(ctx context.Context, namespace string, pvcName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, delOptions)
	return
}

func TransPVCInfo(pvcList *corev1.PersistentVolumeClaimList) (infoList []*kube.PVCInfo) {
	infoList = make([]*kube.PVCInfo, 0)
	for _, pvc := range pvcList.Items {
		quantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		infoList = append(infoList, &kube.PVCInfo{
			Name:     pvc.Name,
			Status:   string(pvc.Status.Phase),
			Capacity: quantity.String(),
		})
	}
	return
}

func WatchPVC(ctx context.Context, namespace string, opts ...options.ListOptionFunc) (watcher watch.Interface, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = kube.GetKubeClient().CoreV1().PersistentVolumeClaims(namespace).
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreatePVCRequest struct {
	Namespace            string                              `json:"namespace"`
	PvcName              string                              `json:"pvc_name"`
	PvcLabels            map[string]string                   `json:"pvc_labels"`
	AccessModes          []corev1.PersistentVolumeAccessMode `json:"access_modes"`
	Storage              string                              `json:"storage"`
	StorageClassName     string                              `json:"storage_class_name"`
	VolumeName           string                              `json:"volume_name"`
	PersistentVolumeMode *corev1.PersistentVolumeMode        `json:"persistent_volume_mode"`
}

func (c *CreatePVCRequest) checkParameters() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if c.PvcName == "" {
		return fmt.Errorf("pvc name is required")
	}

	if len(c.PvcLabels) == 0 {
		c.PvcLabels = map[string]string{
			"pvc": c.PvcName,
		}
	}

	if len(c.AccessModes) == 0 {
		c.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	}
	return nil
}

// CreatePVC 创建PVC
// https://blog.csdn.net/qq_20042935/article/details/129587144
func CreatePVC(ctx context.Context, request CreatePVCRequest) (pvcInfo *corev1.PersistentVolumeClaim, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}
	pvcSrc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: request.PvcName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: request.AccessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse(request.Storage), //设置存储大小
				},
			},
			VolumeName: request.VolumeName,
			VolumeMode: request.PersistentVolumeMode,
		},
	}
	for labelKey, labelVal := range request.PvcLabels {
		pvcSrc.ObjectMeta.Labels[labelKey] = labelVal
	}
	//使用存储卷名字
	if len(request.StorageClassName) != 0 {
		pvcSrc.Spec.StorageClassName = &request.StorageClassName
	}

	options := metav1.CreateOptions{}
	//创建pvc
	pvcInfo, err = kube.GetKubeClient().CoreV1().PersistentVolumeClaims(request.Namespace).Create(ctx, pvcSrc, options)
	if err != nil {
		return nil, err
	}
	return pvcInfo, err
}
