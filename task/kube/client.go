package kube

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"time"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type client struct {
	kc       kubernetes.Interface
	metc     metricsclient.Interface
	ns       string
	host     string
	selector string
}

func NewClient(cfgpath, ns, ingressHost string) (Client, error) {
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

	err = prepareEnvironment(kc, ns)
	if err != nil {
		return nil, errors.Wrap(err, "preparing environment")
	}

	_, err = kc.CoreV1().Namespaces().List(metav1.ListOptions{Limit: 1})
	if err != nil {
		return nil, errors.Wrap(err, "connecting to kubernetes")
	}

	req, err := labels.NewRequirement(managedLabelName, selection.Equals, []string{"true"})
	if err != nil {
		return nil, errors.Wrap(err, "parse selector")
	}
	selector := labels.NewSelector().Add(*req).String()

	return &client{
		kc:       kc,
		metc:     metc,
		ns:       ns,
		host:     ingressHost,
		selector: selector,
	}, nil
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

func (c *client) shouldExpose(expose *types.ManifestServiceExpose) bool {
	return expose.Global &&
		(expose.ExternalPort == 80 ||
			(expose.ExternalPort == 0 && expose.Port == 80))
}

func (c *client) Deploy(group *types.ManifestGroup) error {
	if err := applyNS(c.kc, newNSBuilder(c.ns, group)); err != nil {
		return errors.Wrap(err, "applying namespace")
	}

	if err := cleanupStaleResources(c.kc, c.ns, group); err != nil {
		return errors.Wrap(err, "cleaning stale resources")
	}

	for _, service := range group.Services {
		if err := applyDeployment(c.kc, newDeploymentBuilder(c.ns, group, service)); err != nil {
			return errors.Wrap(err, "applying deployment")
		}

		if len(service.Expose) == 0 {
			continue
		}

		if err := applyService(c.kc, newServiceBuilder(c.ns, group, service)); err != nil {
			return errors.Wrap(err, "applying service")
		}

		for _, expose := range service.Expose {
			if !c.shouldExpose(expose) {
				continue
			}
			if err := applyIngress(c.kc, newIngressBuilder(c.ns, c.host, group, service, expose)); err != nil {
				return errors.Wrap(err, "applying ingress")
			}
		}
	}

	return nil
}

func (c *client) Job(group *types.ManifestGroup) error {
	if err := applyNS(c.kc, newNSBuilder(c.ns, group)); err != nil {
		return errors.Wrap(err, "applying namespace")
	}
	if err := cleanupStaleResources(c.kc, c.ns, group); err != nil {
		return errors.Wrap(err, "cleaning stale resources")
	}

	for _, service := range group.Services {
		if err := applyJob(c.kc, newJobBuilder(c.ns, group, service)); err != nil {
			return errors.Wrap(err, "applying deployment")
		}
	}
	return nil
}
func (c *client) CronJob(group *types.ManifestGroup) error {
	if err := applyNS(c.kc, newNSBuilder(c.ns, group)); err != nil {
		return errors.Wrap(err, "applying namespace")
	}
	if err := cleanupStaleResources(c.kc, c.ns, group); err != nil {
		return errors.Wrap(err, "cleaning stale resources")
	}

	for _, service := range group.Services {
		if err := applyCronJob(c.kc, newcronJobBuilder(c.ns, group, service)); err != nil {
			return errors.Wrap(err, "applying deployment")
		}
	}
	return nil
}

func (c *client) TeardownNamespace() error {
	return c.kc.CoreV1().Namespaces().Delete(c.ns, &metav1.DeleteOptions{})
}

func (c *client) ServiceLogs(ctx context.Context, tailLines int64, follow bool) ([]*ServiceLog, error) {
	pods, err := c.kc.CoreV1().Pods(c.ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "list deployments")
	}
	streams := make([]*ServiceLog, len(pods.Items))
	for i, pod := range pods.Items {
		stream, err := c.kc.CoreV1().Pods(c.ns).GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow:     follow,
			TailLines:  &tailLines,
			Timestamps: true,
		}).Context(ctx).Stream()
		if err != nil {
			return nil, errors.Wrap(err, "get log")
		}
		streams[i] = newServiceLog(pod.Name, stream)
	}
	return streams, nil
}

func (c *client) ListDeployments() ([]string, []string, error) {
	list, err := c.kc.AppsV1().Deployments(c.ns).List(metav1.ListOptions{
		LabelSelector: c.selector,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "list deployments")
	}
	if len(list.Items) == 0 {
		return nil, nil, errors.New("no deployment")
	}

	names := make([]string, 0, len(list.Items))
	contents := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		names = append(names, item.Name)
		content, _ := json.Marshal(item)
		contents = append(contents, string(content))
	}
	return names, contents, nil
}

func (c *client) ServiceStatus(name string) (*types.ServiceStatusResponse, error) {
	deployment, err := c.kc.AppsV1().Deployments(c.ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get deployment")
	}
	if deployment == nil {
		return nil, errors.New("no deployment")
	}
	return &types.ServiceStatusResponse{
		ObservedGeneration: deployment.Status.ObservedGeneration,
		Replicas:           deployment.Status.Replicas,
		UpdatedReplicas:    deployment.Status.UpdatedReplicas,
		ReadyReplicas:      deployment.Status.ReadyReplicas,
		AvailableReplicas:  deployment.Status.AvailableReplicas,
	}, nil
}

func (c *client) Inventory() ([]Node, error) {
	var nodes []Node

	knodes, err := c.activeNodes()
	if err != nil {
		return nil, errors.Wrap(err, "get active nodes")
	}

	mnodes, err := c.metc.MetricsV1beta1().NodeMetricses().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get metrics nodes")
	}

	for _, mnode := range mnodes.Items {
		knode, ok := knodes[mnode.Name]
		if !ok {
			continue
		}

		cpu := knode.Status.Allocatable.Cpu().MilliValue()
		cpu -= mnode.Usage.Cpu().MilliValue()
		if cpu < 0 {
			cpu = 0
		}

		memory := knode.Status.Allocatable.Memory().Value()
		memory -= mnode.Usage.Memory().Value()
		if memory < 0 {
			memory = 0
		}

		disk := knode.Status.Allocatable.StorageEphemeral().Value()
		disk -= mnode.Usage.StorageEphemeral().Value()
		if disk < 0 {
			disk = 0
		}

		unit := types.ResourceUnit{
			CPU:    uint32(cpu),
			Memory: uint64(memory),
			Disk:   uint64(disk),
		}

		nodes = append(nodes, newNode(knode.Name, unit))
	}

	return nodes, nil
}

func (c *client) Metering() (map[string]*types.ResourceUnit, error) {
	list, err := c.kc.CoreV1().Pods(c.ns).List(metav1.ListOptions{
		LabelSelector: c.selector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "list pods")
	}

	now := time.Now()
	result := map[string]*types.ResourceUnit{}
	for _, item := range list.Items {
		var cpu, mem, disk int64
		for _, status := range item.Status.ContainerStatuses {
			if status.State.Running == nil {
				continue
			}

			nano := int64(now.Sub(status.State.Running.StartedAt.Time).Seconds())

			for _, container := range item.Spec.Containers {
				if container.Name != status.Name {
					continue
				}

				limits := container.Resources.Limits
				cpu += nano * limits.Cpu().MilliValue() // unit: types.Core
				mem += nano * limits.Memory().MilliValue() / 1000
				disk += nano * limits.StorageEphemeral().MilliValue() / 1000
			}
			result[status.Name] = &types.ResourceUnit{
				CPU:    uint32(cpu),
				Memory: uint64(mem),
				Disk:   uint64(disk),
			}
		}
	}
	return result, nil
}
func (c *client) activeNodes() (map[string]*corev1.Node, error) {
	knodes, err := c.kc.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get nodes")
	}

	retnodes := make(map[string]*corev1.Node)

	for _, knode := range knodes.Items {
		if !c.nodeIsActive(&knode) {
			continue
		}
		retnodes[knode.Name] = &knode
	}
	return retnodes, nil
}

func (c *client) nodeIsActive(node *corev1.Node) bool {
	ready := false
	issues := 0

	for _, cond := range node.Status.Conditions {
		switch cond.Type {

		case corev1.NodeReady:

			if cond.Status == corev1.ConditionTrue {
				ready = true
			}

		case corev1.NodeOutOfDisk:
			fallthrough
		case corev1.NodeMemoryPressure:
			fallthrough
		case corev1.NodeDiskPressure:
			fallthrough
		case corev1.NodePIDPressure:
			fallthrough
		case corev1.NodeNetworkUnavailable:

			if cond.Status != corev1.ConditionFalse {
				issues++
			}
		}
	}

	return ready && issues == 0
}
