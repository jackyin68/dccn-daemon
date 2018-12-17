package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

func cleanupStaleResources(kc kubernetes.Interface, ns string, group *types.ManifestGroup) error {
	// build label selector for objects not in current manifest group
	svcnames := make([]string, 0, len(group.Services))
	for _, svc := range group.Services {
		svcnames = append(svcnames, svc.Name)
	}

	req1, err := labels.NewRequirement(manifestServiceLabelName, selection.NotIn, svcnames)
	if err != nil {
		return errors.Wrap(err, "service selector")
	}
	req2, err := labels.NewRequirement(managedLabelName, selection.Equals, []string{"true"})
	if err != nil {
		return errors.Wrap(err, "label selector")
	}
	selector := labels.NewSelector().Add(*req1).Add(*req2).String()

	// delete stale deployments
	if err := kc.AppsV1().Deployments(ns).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return errors.Wrap(err, "delete deployment "+ns)
	}

	// delete stale ingresses
	if err := kc.ExtensionsV1beta1().Ingresses(ns).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return errors.Wrap(err, "delete ingress "+ns)
	}

	// delete stale services (no DeleteCollection)
	services, err := kc.CoreV1().Services(ns).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return errors.Wrap(err, "list service "+ns)
	}
	for _, svc := range services.Items {
		if err := kc.CoreV1().Services(ns).Delete(svc.Name, &metav1.DeleteOptions{}); err != nil {
			return errors.Wrapf(err, "delete service %s %s", ns, svc.Name)
		}
	}

	return nil
}
