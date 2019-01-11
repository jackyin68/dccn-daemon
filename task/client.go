package task

import (
	"os"
	"path"

	"github.com/Ankr-network/dccn-daemon/task/kube"
	"github.com/pkg/errors"
	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Client struct {
	kc   kubernetes.Interface
	metc metricsclient.Interface
	ns   string
	host string
}

func NewClient(cfgpath, ns, ingressHost string) (*Client, error) {
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

	return &Client{
		kc:   kc,
		metc: metc,
		ns:   ns,
		host: ingressHost,
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

func (c *Client) run(kubes []kube.Kube) error {
	rollbacks := make([]func(kubernetes.Interface) error, len(kubes))
	for i := range kubes {
		rollback, err := kubes[i].Update(c.kc)
		if k8sErr.IsNotFound(err) {
			if err := kubes[i].Create(c.kc); err != nil {
				return c.rollback(rollbacks, err)
			}
			rollback = kubes[i].Delete

		} else if err != nil {
			return c.rollback(rollbacks, err)
		}

		rollbacks = append(rollbacks, rollback)
	}
	return nil
}
func (c *Client) rollback(rollbacks []func(kubernetes.Interface) error, err error) error {
	for i := len(rollbacks); i >= 0; i-- {
		if e := rollbacks[i](c.kc); e != nil {
			return errors.WithMessage(err, "rollback: "+e.Error())
		}
	}
	return errors.WithMessage(err, "rolled back")
}
