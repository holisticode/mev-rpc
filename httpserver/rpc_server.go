package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/flashbots/go-utils/rpcserver"
	"github.com/holisticode/mev-rpc/database"
)

const (
	RPCModuleByTX    = "mev_rpc_tx"
	RPCModuleByBlock = "mev_rpc_block"
)

type MEVJSONRPCServer struct {
	*http.Server
	dbService database.MEVTraceStorage
	log       *slog.Logger
}

func NewJSONRPCServer(cfg *HTTPServerConfig) (*http.Server, error) {
	mevServer := &MEVJSONRPCServer{
		dbService: cfg.DBService,
		log:       cfg.Log,
	}
	methods := map[string]any{
		RPCModuleByBlock: mevServer.handleByBlock,
		RPCModuleByTX:    mevServer.handleByTx,
	}
	opts := rpcserver.JSONRPCHandlerOpts{}
	handler, err := rpcserver.NewJSONRPCHandler(methods, opts)
	if err != nil {
		return nil, fmt.Errorf("failed creating JSONRPCHandler: %w", err)
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
	s.log.Debug("MEVJSONRPCServer handleByTx", "tx", tx)
	return s.dbService.GetMEVTx(tx)
}

func (s *MEVJSONRPCServer) handleByBlock(ctx context.Context, block string) (*database.MEVBlock, error) {
	s.log.Debug("MEVJSONRPCServer handleByBlock", "block", block)
	return s.dbService.GetMEVBlock(block)
}
