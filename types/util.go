package types

import (
	"github.com/Ankr-network/dccn-daemon/types/unit"
	corev1 "k8s.io/api/core/v1"
)

//NewManifestService is a easy way to create a manifest service
func NewManifestService(name, image string) *ManifestService {
	return &ManifestService{
		Name:  name,
		Image: image,
		Args:  []string{},
		Env:   []string{},
		Unit: &ResourceUnit{
			CPU:    unit.Core / 10,
			Memory: 128 * unit.Mi,
			Disk:   256 * unit.Mi,
		},
		Count: 1,
		Expose: []*ManifestServiceExpose{{
			Port:         80,
			ExternalPort: 80,
			Proto:        string(corev1.ProtocolTCP),
			Service:      name,
			Global:       true,
			Hosts:        []string{name},
		}},
	}
}

//NewJobManifestService is a easy way to create a manifest service for job
func NewJobManifestService(name, image string) *ManifestService {
	return &ManifestService{
		Name:  name,
		Image: image,
		Args:  []string{},
		Env:   []string{},
		Unit: &ResourceUnit{
			CPU:    unit.Core / 10,
			Memory: 128 * unit.Mi,
			Disk:   256 * unit.Mi,
		},
		Count: 1,
	}
}
