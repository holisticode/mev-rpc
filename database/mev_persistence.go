package database

import "log/slog"

// MEVTraceStorage groups functions we need for this work test
type MEVTraceStorage interface {
	LatestBlock() (uint64, error)
	GetMEVTx(tx string) (*MEVTransaction, error)
	GetMEVBlock(block string) (*MEVBlock, error)
	OldestBlock() uint64
	SaveMEVBLock(block *MEVBlock, txs []*MEVTransaction) error //nolint:ireturn
}

// NewStorage returns the service to store the data
func NewStorage(conn string, log *slog.Logger) (*DatabaseService, error) {
	return NewDatabaseService(conn, log)
}
