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

// MEVJSONRPCServer is used to run thiw work task's RPC server
type MEVJSONRPCServer struct {
	*http.Server
	// the JSON RPC server needs to interface with the DB to get data
	dbService database.MEVTraceStorage
	log       *slog.Logger
}

// NewJSONRPCServer creates a new one
func NewJSONRPCServer(cfg *HTTPServerConfig) (*http.Server, error) {
	mevServer := &MEVJSONRPCServer{
		dbService: cfg.DBService,
		log:       cfg.Log,
	}
	// our 2 methods supported by this RPC server
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

// handleByTx() is simple, just calls the DB Service with the appropriate method
func (s *MEVJSONRPCServer) handleByTx(ctx context.Context, tx string) (*database.MEVTransaction, error) {
	s.log.Debug("MEVJSONRPCServer handleByTx", "tx", tx)
	return s.dbService.GetMEVTx(tx)
}

// handleByBlock() is simple, just calls the DB Service with the appropriate method
func (s *MEVJSONRPCServer) handleByBlock(ctx context.Context, block string) (*database.MEVBlock, error) {
	s.log.Debug("MEVJSONRPCServer handleByBlock", "block", block)
	return s.dbService.GetMEVBlock(block)
}
