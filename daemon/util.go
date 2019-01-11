package daemon

//go:generate stringer -type taskType
type taskType int

// To do: move these definition into DCCN-common
const (
	NewTask taskType = iota
	UpdateTask
	CancelTask
	ListTask
	HeartBeat
)

//go:generate stringer -type msgStatus
type msgStatus int

const (
	StartFailure msgStatus = iota
	StartSuccess
	CancelFailure
	Cancelled
	UpdateFailure
	UpdateSuccess
)

// TendermintKey will create the key to be used in the tendermint
func TendermintKey(dcName, namespace string) string {
	return dcName + ":" + namespace
}
