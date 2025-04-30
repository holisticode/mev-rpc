package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flashbots/go-utils/rpcserver"
)

const (
	RPC_MODULE_BY_TX    = "rpc-module-name_tx"
	RPC_MODULE_BY_BLOCK = "rpc-module-name_block"
)

func NewJSONRPCServer(cfg *HTTPServerConfig) (*http.Server, error) {
	methods := map[string]interface{}{
		RPC_MODULE_BY_BLOCK: handleByBlock,
		RPC_MODULE_BY_TX:    handleByTx,
	}
	opts := rpcserver.JSONRPCHandlerOpts{}
	handler, err := rpcserver.NewJSONRPCHandler(methods, opts)
	if err != nil {
		return nil, fmt.Errorf("failed creating JSONRPCHandler: %v", err)
	}
	s := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s, nil
}

func handleByTx(ctx context.Context, arg1 int) (bool, error) {
	return true, nil
}

func handleByBlock(ctx context.Context, arg1 int) (bool, error) {
	return true, nil
}
