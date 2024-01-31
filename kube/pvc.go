package kube

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type ListOptionFunc func(options *metav1.ListOptions)

func WithLabelSelector(label string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = label
	}
}

func WithFieldSelector(field string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = field
	}
}
func WithLimitCount(limit int64) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Limit = limit
	}
}

func WithContinues(continues string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Continue = continues
	}
}

func WithWatch(watch bool) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.Watch = watch
	}
}

func WithResourceVersion(version string) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.ResourceVersion = version
	}
}

func WithSendInitialEvents(sendInitialEvents bool) ListOptionFunc {
	return func(options *metav1.ListOptions) {
		options.SendInitialEvents = &sendInitialEvents
	}
}

func GetPVCList(ctx context.Context, namespace string, opts ...ListOptionFunc) (pvcList *corev1.PersistentVolumeClaimList, err error) {
	apiV1 := GetKubeClient().CoreV1()
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	pvcList, err = apiV1.PersistentVolumeClaims(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetPVCInfo(ctx context.Context, namespace string, pvcName string) (pvcInfo *corev1.PersistentVolumeClaim, err error) {
	apiV1 := GetKubeClient().CoreV1()
	getOptions := metav1.GetOptions{}
	pvcInfo, err = apiV1.PersistentVolumeClaims(namespace).Get(ctx, pvcName, getOptions)
	return
}

func TransPVCInfo(pvcList *corev1.PersistentVolumeClaimList) (infoList []*PVCInfo) {
	infoList = make([]*PVCInfo, 0)
	for _, pvc := range pvcList.Items {
		quantity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		infoList = append(infoList, &PVCInfo{
			Name:     pvc.Name,
			Status:   string(pvc.Status.Phase),
			Capacity: quantity.String(),
		})
	}
	return
}

func WatchPVC(ctx context.Context, namespace string, opts ...ListOptionFunc) (watcher watch.Interface, err error) {
	apiV1 := GetKubeClient().CoreV1()
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	watcher, err = apiV1.PersistentVolumeClaims(namespace).
		Watch(ctx, listOptions)
	if err != nil {
		return
	}
	return
}

type CreatePVCRequest struct {
	NamespaceName    string                              `json:"namespace_name"`
	PvcName          string                              `json:"pvc_name"`
	AccessModes      []corev1.PersistentVolumeAccessMode `json:"access_modes"`
	StorageSize      int64                               `json:"storage_size"`
	StorageClassName string                              `json:"storage_class_name"`
}

func CreatePVC(ctx context.Context, request CreatePVCRequest) (pvcInfo *corev1.PersistentVolumeClaim, err error) {

	pvcSrc := new(corev1.PersistentVolumeClaim)
	pvcSrc.ObjectMeta.Name = request.PvcName
	pvcSrc.Spec.AccessModes = request.AccessModes

	//设置存储大小
	var resourceQuantity resource.Quantity
	resourceQuantity.Set(request.StorageSize * 1000 * 1000 * 1000)
	pvcSrc.Spec.Resources.Requests = corev1.ResourceList{
		"storage": resourceQuantity,
	}

	//使用存储卷名字
	if len(request.StorageClassName) != 0 {
		pvcSrc.Spec.StorageClassName = &request.StorageClassName
	}

	options := metav1.CreateOptions{}
	//创建pvc
	pvcInfo, err = GetKubeClient().CoreV1().PersistentVolumeClaims(request.NamespaceName).Create(ctx, pvcSrc, options)
	if err != nil {
		return nil, err
	}
	return pvcInfo, err
}
