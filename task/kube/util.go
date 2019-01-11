package kube

import (
	"strings"

	"github.com/Ankr-network/dccn-daemon/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

// For unit test
var mockKube Kube

// Kubenetes actions interface
//go:generate mockgen -package $GOPACKAGE -destination mock_kube.go github.com/Ankr-network/dccn-daemon/task/kube Kube
type Kube interface {
	Create(kc kubernetes.Interface) (err error)
	Delete(kc kubernetes.Interface) (err error)
	Update(kc kubernetes.Interface) (rollback func(kubernetes.Interface) error, err error)
	DeleteCollection(kc kubernetes.Interface, options metav1.ListOptions) (err error)
	List(kc kubernetes.Interface, result interface{}) (err error)
}

const managedLabelName = "ankr.network"
const manifestServiceLabelName = "ankr.network/manifest-service"

type common struct {
	namespace string
	service   *types.ManifestService
}

func (c *common) ns() string {
	return c.namespace
}
func (c *common) name() string {
	return c.service.Name
}
func (c *common) labels() map[string]string {
	return map[string]string{
		managedLabelName: "true",
	}
}

func (c *common) container() corev1.Container {
	qcpu := resource.NewScaledQuantity(int64(c.service.Unit.CPU), resource.Milli)
	qmem := resource.NewQuantity(int64(c.service.Unit.Memory), resource.DecimalSI)
	qdisk := resource.NewQuantity(int64(c.service.Unit.Disk), resource.DecimalSI)

	kcontainer := corev1.Container{
		Name:  c.service.Name,
		Image: c.service.Image,
		Args:  c.service.Args,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              qcpu.DeepCopy(),
				corev1.ResourceMemory:           qmem.DeepCopy(),
				corev1.ResourceEphemeralStorage: qdisk.DeepCopy(),
			},
		},
	}

	for _, env := range c.service.Env {
		parts := strings.Split(env, "=")
		switch len(parts) {
		case 2:
			kcontainer.Env = append(kcontainer.Env, corev1.EnvVar{Name: parts[0], Value: parts[1]})
		case 1:
			kcontainer.Env = append(kcontainer.Env, corev1.EnvVar{Name: parts[0]})
		}
	}

	for _, expose := range c.service.Expose {
		kcontainer.Ports = append(kcontainer.Ports, corev1.ContainerPort{
			ContainerPort: int32(expose.Port),
		})
	}

	return kcontainer
}

func exposeExternalPort(expose *types.ManifestServiceExpose) int32 {
	if expose.ExternalPort == 0 {
		return int32(expose.Port)
	}
	return int32(expose.ExternalPort)
}

func Selector() string {
	req, _ := labels.NewRequirement(managedLabelName, selection.Equals, []string{"true"})
	return labels.NewSelector().Add(*req).String()
}
