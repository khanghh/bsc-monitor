package leth

import (
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	rpcDialRetryDelay = 1 * time.Second
)

// RpcConnector proviode a thread-safe rpc connection
type RpcConnector struct {
	client     *rpc.Client
	rpcUrl     string
	maxAttempt int
}

func (c *RpcConnector) Client() *rpc.Client {
	return c.client
}

func (c *RpcConnector) RpcUrl() string {
	return c.rpcUrl
}

// SetMaxAttempt set maximum number of attempt to dial rpc node
// provide a number less than or equal 0 to repeat dialing until successful
func (c *RpcConnector) SetMaxAttempt(max int) {
	c.maxAttempt = max
}

// Connect repeats dial rpc node util max attempt has been reached
// if max attempt has not been specify, dialing will repeat until successful
func (c *RpcConnector) Connect() (*rpc.Client, error) {
	var (
		err     error
		attempt int
	)
	for {
		attempt += 1
		log.Debug("Dialing RPC node...", "rpcUrl", c.rpcUrl, "attempt", attempt)
		c.client, err = rpc.Dial(c.rpcUrl)
		if err == nil {
			return c.client, nil
		}
		if c.maxAttempt > 0 && attempt <= c.maxAttempt {
			return nil, err
		}
		time.Sleep(rpcDialRetryDelay)
	}
}

func NewRpcConnector(rpcUrl string) *RpcConnector {
	return &RpcConnector{rpcUrl: rpcUrl}
}
