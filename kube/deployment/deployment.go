package deployment

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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

func GetDeploymentInfo(ctx context.Context, namespace string, deploymentName string) (deploymentInfo *appsv1.Deployment, err error) {
	getOptions := metav1.GetOptions{}
	deploymentInfo, err = kube.GetKubeClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, getOptions)
	return
}

func DelDeployment(ctx context.Context, namespace string, deploymentName string) (err error) {
	delOptions := metav1.DeleteOptions{}
	err = kube.GetKubeClient().AppsV1().Deployments(namespace).Delete(ctx, deploymentName, delOptions)
	return
}

type CreateDeploymentRequest struct {
	Namespace             string            `json:"namespace"`
	DeploymentName        string            `json:"deployment_name"`
	Annotations           map[string]string `json:"annotations"`
	RevisionHistoryLimit  int32             `json:"revision_history_limit"`
	Replicas              int32             `json:"replicas"`
	InitContainerInfoList []ContainerInfo   `json:"init_container_info_list"`
	ContainerInfoList     []ContainerInfo   `json:"container_info_list"`
}

type MountConfigmap struct {
	FileName      string `json:"file_name"`
	MountPath     string `json:"mount_path"`
	ConfigmapName string `json:"configmap_name"`
}

type ContainerInfo struct {
	Name                   string           `json:"name"`
	Image                  string           `json:"image"`
	Command                []string         `json:"command"`
	Args                   []string         `json:"args"`
	WorkingDir             string           `json:"workingDir"`
	ContainerPorts         []int32          `json:"container_ports"`
	Env                    []EnvVar         `json:"env"`
	Resources              *ResourceConfig  `json:"resources"`
	Stdin                  bool             `json:"stdin"`
	StdinOnce              bool             `json:"stdinOnce"`
	TTY                    bool             `json:"tty"`
	MountConfigmapInfoList []MountConfigmap `json:"mount_configmap_info_list"`
	MountPvcInfoList       []MountPvc       `json:"mount_pvc_info_list"`
	HostPathInfoList       []MountHostPath  `json:"host_path_info_list"`
}

type ResourceConfig struct {
	CPU    *ResourceInfo
	Memory *ResourceInfo
}

type ResourceInfo struct {
	Limit   string
	Request string
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type MountPvc struct {
	MountPath string `json:"mount_path"`
	PvcName   string `json:"pvc_name"`
}

type MountHostPath struct {
	MountPodPath  string `json:"mount_pod_path"`
	MountHostPath string `json:"mount_host_path"`
	Tag           string `json:"tag"`
}

func (c *CreateDeploymentRequest) checkParameters() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}

	if c.DeploymentName == "" {
		return fmt.Errorf("deployment name is required")
	}

	if c.Replicas == 0 {
		c.Replicas = 1
	}

	if c.RevisionHistoryLimit == 0 {
		c.RevisionHistoryLimit = 3
	}

	for index, initContainer := range c.InitContainerInfoList {
		if initContainer.Name == "" {
			initContainer.Name = fmt.Sprintf("%s_container_%d", c.DeploymentName, index)
		}

		if initContainer.Image == "" {
			return fmt.Errorf("init container's image is required")
		}

		if initContainer.Resources == nil {
			initContainer.Resources = &ResourceConfig{
				CPU: &ResourceInfo{
					Limit:   "1",
					Request: "0.5",
				},
				Memory: &ResourceInfo{
					Limit:   "1Gi",
					Request: "500Mi",
				},
			}
		} else {
			if initContainer.Resources.CPU == nil {
				initContainer.Resources.CPU = &ResourceInfo{
					Limit:   "1",
					Request: "0.5",
				}
			}
			if initContainer.Resources.Memory == nil {
				initContainer.Resources.Memory = &ResourceInfo{
					Limit:   "1Gi",
					Request: "500Mi",
				}
			}
		}
	}

	for index, container := range c.ContainerInfoList {
		if container.Name == "" {
			container.Name = fmt.Sprintf("%s_container_%d", c.DeploymentName, index)
		}

		if container.Image == "" {
			return fmt.Errorf("container's image is required")
		}

		if container.Resources == nil {
			container.Resources = &ResourceConfig{
				CPU: &ResourceInfo{
					Limit:   "1",
					Request: "0.5",
				},
				Memory: &ResourceInfo{
					Limit:   "1Gi",
					Request: "500Mi",
				},
			}
		} else {
			if container.Resources.CPU == nil {
				container.Resources.CPU = &ResourceInfo{
					Limit:   "1",
					Request: "0.5",
				}
			}
			if container.Resources.Memory == nil {
				container.Resources.Memory = &ResourceInfo{
					Limit:   "1Gi",
					Request: "500Mi",
				}
			}
		}
	}

	return nil
}

func CreateDeployment(ctx context.Context, request *CreateDeploymentRequest) (deploymentInfo *appsv1.Deployment, err error) {
	err = request.checkParameters()
	if err != nil {
		return nil, err
	}

	containers := make([]corev1.Container, 0)
	initContainers := make([]corev1.Container, 0)
	volumes := make([]corev1.Volume, 0)
	for _, info := range request.InitContainerInfoList {
		containerPort := explainPort(info.ContainerPorts)
		//调用解释器（之后定义）
		volumeMounts, volumeItems := explainVolumeMounts(info.MountConfigmapInfoList, info.MountPvcInfoList, info.HostPathInfoList)
		volumes = append(volumes, volumeItems...)
		initContainers = append(initContainers, corev1.Container{
			Name:    info.Name,
			Command: info.Command,
			Args:    info.Args,
			Env: func() []corev1.EnvVar {
				envList := make([]corev1.EnvVar, 0)
				for _, envVar := range info.Env {
					envList = append(envList, corev1.EnvVar{
						Name:  envVar.Name,
						Value: envVar.Value,
					})
				}
				return envList
			}(),
			Image:           info.Image,
			ImagePullPolicy: "Always",
			Ports:           containerPort,
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
			LivenessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				TimeoutSeconds:      5,
				PeriodSeconds:       30,
				SuccessThreshold:    1,
				FailureThreshold:    5,
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    5,
			},
			StartupProbe:    &corev1.Probe{},
			VolumeMounts:    volumeMounts,
			SecurityContext: nil,
			Stdin:           info.Stdin,
			StdinOnce:       info.StdinOnce,
			TTY:             info.TTY,
		})
	}
	for _, info := range request.ContainerInfoList {
		containerPort := explainPort(info.ContainerPorts)
		//调用解释器（之后定义）
		volumeMounts, volumeItems := explainVolumeMounts(info.MountConfigmapInfoList, info.MountPvcInfoList, info.HostPathInfoList)
		volumes = append(volumes, volumeItems...)
		containers = append(containers, corev1.Container{
			Name:    info.Name,
			Command: info.Command,
			Args:    info.Args,
			Env: func() []corev1.EnvVar {
				envList := make([]corev1.EnvVar, 0)
				for _, envVar := range info.Env {
					envList = append(envList, corev1.EnvVar{
						Name:  envVar.Name,
						Value: envVar.Value,
					})
				}
				return envList
			}(),
			Image:           info.Image,
			ImagePullPolicy: "Always",
			Ports:           containerPort,
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
			LivenessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				TimeoutSeconds:      5,
				PeriodSeconds:       50,
				SuccessThreshold:    1,
				FailureThreshold:    15,
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 30,
				TimeoutSeconds:      5,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    15,
			},
			StartupProbe: &corev1.Probe{},
			VolumeMounts: volumeMounts,
			Stdin:        info.Stdin,
			StdinOnce:    info.StdinOnce,
			TTY:          info.TTY,
		})
	}
	//这个结构和原生k8s启动deployment的yml文件结构完全一样，对着写就好
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.DeploymentName,
			Namespace:   request.Namespace,
			Annotations: request.Annotations,
			Labels: map[string]string{
				"app":     request.DeploymentName,
				"version": "v1",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &request.Replicas,
			RevisionHistoryLimit: &request.RevisionHistoryLimit,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":     request.DeploymentName,
					"version": "v1",
				},
			},
			MinReadySeconds: 10,
			Strategy: appsv1.DeploymentStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 1,
					},
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 0,
					},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.DeploymentName,
					Namespace: request.Namespace,
					Labels: map[string]string{
						"app":     request.DeploymentName,
						"version": "v1",
					},
				},
				Spec: corev1.PodSpec{
					Volumes:        volumes,
					InitContainers: initContainers,
					Containers:     containers,
					HostNetwork:    false,
					RestartPolicy:  "Always",
					DNSPolicy:      "ClusterFirstWithHostNet",
				},
			},
		},
	}
	//创建deployment
	deploymentInfo, err = kube.GetKubeClient().AppsV1().Deployments(request.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return deploymentInfo, err
	}
	return deploymentInfo, nil
}

func explainPort(ports []int32) (portInfoList []corev1.ContainerPort) {
	for _, port := range ports {
		portInfoList = append(portInfoList, corev1.ContainerPort{ContainerPort: port})
	}
	return portInfoList
}

// 定义一个总的解释器，调用各子解释器
func explainVolumeMounts(mountConfigmapInfoList []MountConfigmap, mountPvcInfoList []MountPvc, hostPathInfoList []MountHostPath) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {

	//解释configmap
	if len(mountConfigmapInfoList) != 0 {
		volumeMounts1, volumes1 := explainConfigmap(mountConfigmapInfoList)
		volumeMounts = append(volumeMounts, volumeMounts1...)
		volumes = append(volumes, volumes1...)
	}

	//解释目录挂载PVC
	if len(mountPvcInfoList) != 0 {
		volumeMounts2, volumes2 := explainPvc(mountPvcInfoList)
		volumeMounts = append(volumeMounts, volumeMounts2...)
		volumes = append(volumes, volumes2...)
	}

	//解释挂载本机目录
	if len(hostPathInfoList) != 0 {
		volumeMounts3, volumes3 := explainHostPath(hostPathInfoList)
		volumeMounts = append(volumeMounts, volumeMounts3...)
		volumes = append(volumes, volumes3...)
	}

	return volumeMounts, volumes
}

// 定义configmap的解释器
func explainConfigmap(mountConfigmapInfoList []MountConfigmap) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range mountConfigmapInfoList {
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
func explainPvc(mountPvcInfoList []MountPvc) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range mountPvcInfoList {
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
func explainHostPath(hostPathInfo []MountHostPath) (volumeMounts []corev1.VolumeMount, volumes []corev1.Volume) {
	for _, mountInfo := range hostPathInfo {
		//拼接 volumeMount.Name
		volumeMountName := mountInfo.Tag
		//给volumeMount赋值
		volumeMount := corev1.VolumeMount{
			Name:      volumeMountName,
			MountPath: mountInfo.MountPodPath,
		}
		volumeMounts = append(volumeMounts, volumeMount)

		//给volume赋值
		volume := corev1.Volume{
			Name: volumeMountName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: mountInfo.MountHostPath,
				},
			},
		}
		volumes = append(volumes, volume)
	}
	return volumeMounts, volumes
}
