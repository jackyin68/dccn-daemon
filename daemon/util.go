package daemon

// TendermintKey will create the key to be used in the tendermint
func TendermintKey(dcName, namespace string) string {
	return dcName + ":" + namespace
}
