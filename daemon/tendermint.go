package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/types"
)

func Broadcast(server, wsEndpoint, key string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return errors.Wrap(err, "json marshal")
	}

	c := client.NewHTTP(server, wsEndpoint)
	_, err = c.BroadcastTxCommit(types.Tx(
		fmt.Sprintf("%s=%s", key, data)))
	return errors.Wrap(err, "broadcast")
}
