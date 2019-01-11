package types

import (
	"bytes"
	"strconv"

	"github.com/Ankr-network/dccn-daemon/types/base"
)

func (id DeploymentGroupID) String() string {
	return id.Deployment.String() + "/" + strconv.FormatUint(id.Seq, 10)
}

func (id DeploymentGroupID) Path() string {
	return id.String()
}

func (id DeploymentGroupID) Compare(that interface{}) int {
	switch that := that.(type) {
	case DeploymentGroupID:
		if cmp := bytes.Compare(id.Deployment, that.Deployment); cmp != 0 {
			return cmp
		}
		return int(id.Seq - that.Seq)
	case *DeploymentGroupID:
		if cmp := bytes.Compare(id.Deployment, that.Deployment); cmp != 0 {
			return cmp
		}
		return int(id.Seq - that.Seq)
	default:
		return 1
	}
}

func (id DeploymentGroupID) DeploymentID() base.Bytes {
	return id.Deployment
}

func (r ResourceUnit) Equal(that interface{}) bool {
	return (&r).Compare(that) == 0
}
