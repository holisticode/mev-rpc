package httpserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/flashbots/go-utils/rpcserver"
	"github.com/holisticode/mev-rpc/database"
)

const (
	RPC_MODULE_BY_TX    = "mev_rpc_tx"
	RPC_MODULE_BY_BLOCK = "mev_rpc_block"
)

type MEVJSONRPCServer struct {
	*http.Server
	dbService database.MEVTraceStorage
}

func NewJSONRPCServer(cfg *HTTPServerConfig) (*http.Server, error) {
	mevServer := &MEVJSONRPCServer{
		dbService: cfg.DBService,
	}
	methods := map[string]any{
		RPC_MODULE_BY_BLOCK: mevServer.handleByBlock,
		RPC_MODULE_BY_TX:    mevServer.handleByTx,
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
	mevServer.Server = s
	return s, nil
}

func (s *MEVJSONRPCServer) handleByTx(ctx context.Context, tx string) (*database.MEVTransaction, error) {
	mev_tx, err := s.dbService.GetMEVTx(tx)
	if err != nil {
		return nil, err
	}
	return mev_tx, nil
}

func (s *MEVJSONRPCServer) handleByBlock(ctx context.Context, arg1 int) (bool, error) {
	return true, nil
}
