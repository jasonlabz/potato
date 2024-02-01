package deployment

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	"github.com/jasonlabz/potato/kube"
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

func GetDeploymentList(ctx context.Context, namespace string, opts ...ListOptionFunc) (deploymentList *appsv1.DeploymentList, err error) {
	listOptions := metav1.ListOptions{}
	for _, opt := range opts {
		opt(&listOptions)
	}
	deploymentList, err = kube.GetKubeClient().AppsV1().Deployments(namespace).List(ctx, listOptions)
	return
}

type GetOptionFunc func(options *metav1.GetOptions)

func GetDeploymentInfo(ctx context.Context, namespace string, deploymentName string) (deploymentInfo *appsv1.Deployment, err error) {
	getOptions := metav1.GetOptions{}
	deploymentInfo, err = kube.GetKubeClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, getOptions)
	return
}

type CreateDeploymentRequest struct {
	NamespaceName     string          `json:"namespace_name"`
	DeploymentName    string          `json:"deployment_name"`
	Replicas          int32           `json:"replicas"`
	ContainerInfoList []ContainerInfo `json:"container_info_list"`
}

type MountConfigmapInfo struct {
	FileName      string `json:"file_name"`
	MountPath     string `json:"mount_path"`
	ConfigmapName string `json:"configmap_name"`
}

type ContainerInfo struct {
	Name                   string               `json:"name"`
	Image                  string               `json:"image"`
	Command                []string             `json:"command"`
	Args                   []string             `json:"args"`
	WorkingDir             string               `json:"workingDir"`
	ContainerPorts         []int32              `json:"container_ports"`
	Env                    []EnvVar             `json:"env"`
	Resources              ResourceConfig       `json:"resources"`
	Stdin                  bool                 `json:"stdin"`
	StdinOnce              bool                 `json:"stdinOnce"`
	TTY                    bool                 `json:"tty"`
	MountConfigmapInfoList []MountConfigmapInfo `json:"mount_configmap_info_list"`
	MountPvcInfoList       []MountPvcInfo       `json:"mount_pvc_info_list"`
	HostPathInfoList       []HostPathInfo       `json:"host_path_info_list"`
}

type ResourceConfig struct {
	CPU    ResourceInfo
	Memory ResourceInfo
}

type ResourceInfo struct {
	Limit   string
	Request string
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type MountPvcInfo struct {
	MountPath string `json:"mount_path"`
	PvcName   string `json:"pvc_name"`
}

type HostPathInfo struct {
	MountPathIn  string `json:"mount_path_in"`
	MountPathOut string `json:"mount_path_out"`
	Tag          string `json:"tag"`
}

func CreateDeployment(ctx context.Context, request CreateDeploymentRequest) (deploymentInfo *appsv1.Deployment, err error) {
	containers := make([]corev1.Container, 0)
	volumes := make([]corev1.Volume, 0)
	for _, info := range request.ContainerInfoList {
		containerPort := explainPort(info.ContainerPorts)
		//调用解释器（之后定义）
		volumeMounts, volumeItems := explainVolumeMounts(info.MountConfigmapInfoList, info.MountPvcInfoList, info.HostPathInfoList)
		volumes = append(volumes, volumeItems...)
		containers = append(containers, corev1.Container{
			Name:  request.DeploymentName,
			Image: info.Image,
			Ports: containerPort,
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(info.Resources.CPU.Limit),
					corev1.ResourceMemory: resource.MustParse(info.Resources.Memory.Limit),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(info.Resources.CPU.Request),
					corev1.ResourceMemory: resource.MustParse(info.Resources.Memory.Request),
				},
				Claims: make([]corev1.ResourceClaim, 0),
			},
			VolumeMounts: volumeMounts,
		})
	}
	//这个结构和原生k8s启动deployment的yml文件结构完全一样，对着写就好
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.DeploymentName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &request.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": request.DeploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": request.DeploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Volumes:    volumes,
					Containers: containers,
				},
			},
		},
	}
	//创建deployment
	deploymentInfo, err = kube.GetKubeClient().AppsV1().Deployments(request.NamespaceName).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return deploymentInfo, err
	}
	return deploymentInfo, nil
}

func explainPort(ports []int32) (portInfos []corev1.ContainerPort) {
	for _, port := range ports {
		portInfos = append(portInfos, corev1.ContainerPort{ContainerPort: port})
	}
	return portInfos
}

// 定义一个总的解释器，调用各子解释器
func explainVolumeMounts(mountConfigmapInfos []MountConfigmapInfo, mountPvcInfos []MountPvcInfo, hostPathInfos []HostPathInfo) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {

	//解释configmap
	if len(mountConfigmapInfos) != 0 {
		volumeMounts1, volumes1 := explainConfigmap(mountConfigmapInfos)
		volumeMounts = append(volumeMounts, volumeMounts1...)
		volumes = append(volumes, volumes1...)
	}

	//解释目录挂载PVC
	if len(mountPvcInfos) != 0 {
		volumeMounts2, volumes2 := explainPvc(mountPvcInfos)
		volumeMounts = append(volumeMounts, volumeMounts2...)
		volumes = append(volumes, volumes2...)
	}

	//解释挂载本机目录
	if len(hostPathInfos) != 0 {
		volumeMounts3, volumes3 := explainHostPath(hostPathInfos)
		volumeMounts = append(volumeMounts, volumeMounts3...)
		volumes = append(volumes, volumes3...)
	}

	return volumeMounts, volumes
}

// 定义configmap的解释器
func explainConfigmap(mountConfigmapInfos []MountConfigmapInfo) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range mountConfigmapInfos {
		//拼接 volumeMount.Name
		volumeMountNameSuffix := strings.Split(mountInfo.FileName, ".")[0]
		volumeMountName := mountInfo.ConfigmapName + volumeMountNameSuffix
		//给volumeMount赋值
		volumeMount := corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: mountInfo.MountPath + "/" + mountInfo.FileName,
			SubPath:   mountInfo.FileName,
		}
		volumeMounts = append(volumeMounts, volumeMount)

		//给volume赋值
		volume := corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mountInfo.ConfigmapName,
					},
				},
			},
		}
		volumes = append(volumes, volume)
	}
	return volumeMounts, volumes
}

// 定义pvc的解释器
func explainPvc(mountPvcInfos []MountPvcInfo) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range mountPvcInfos {
		//拼接 volumeMount.Name
		volumeMountName := mountInfo.PvcName
		//给volumeMount赋值
		volumeMount := corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: mountInfo.MountPath,
		}
		volumeMounts = append(volumeMounts, volumeMount)

		//给volume赋值
		volume := corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: mountInfo.PvcName,
				},
			},
		}
		volumes = append(volumes, volume)
	}
	return volumeMounts, volumes
}

// 定义hostpath的解释器
func explainHostPath(hostPathInfo []HostPathInfo) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range hostPathInfo {
		//拼接 volumeMount.Name
		volumeMountName := mountInfo.Tag
		//给volumeMount赋值
		volumeMount := corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: mountInfo.MountPathIn,
		}
		volumeMounts = append(volumeMounts, volumeMount)

		//给volume赋值
		volume := corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountInfo.MountPathOut,
				},
			},
		}
		volumes = append(volumes, volume)
	}
	return volumeMounts, volumes
}
