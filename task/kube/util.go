package kube

import (
	"os"
	"path"
	"strings"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// For unit test
var mockKube Kube

// Kubernetes actions interface
//go:generate mockgen -package $GOPACKAGE -destination mock_kube.go github.com/Ankr-network/dccn-daemon/task/kube Kube
type Kube interface {
	Create(c *Client) (err error)
	Delete(c *Client) (err error)
	Update(c *Client) (rollback func(c *Client) error, err error)
	DeleteCollection(c *Client, options metav1.ListOptions) (err error)
	List(c *Client, result interface{}) (err error)
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
func IsNotFound(err error) bool {
	if err != nil {
		return strings.HasSuffix(err.Error(), "not found")
	}
	return false
}

type Client struct {
	kubernetes.Interface
	metc metricsclient.Interface
}

func NewClient(cfgpath string) (*Client, error) {
	config, err := openKubeConfig(cfgpath)
	if err != nil {
		return nil, errors.Wrap(err, "building config flags")
	}

	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "creating kubernetes client")
	}

	metc, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "creating metrics client")
	}

	return &Client{kc, metc}, nil
}
func openKubeConfig(cfgpath string) (*rest.Config, error) {
	if cfgpath == "" {
		cfgpath = path.Join(homedir.HomeDir(), ".kube", "config")
	}

	if _, err := os.Stat(cfgpath); err == nil {
		cfg, err := clientcmd.BuildConfigFromFlags("", cfgpath)
		if err != nil {
			return nil, errors.Wrap(err, cfgpath)
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, cfgpath+" fallback in cluster")
	}
	return cfg, nil
}
