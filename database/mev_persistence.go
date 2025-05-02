package database

type MEVTraceStorage interface {
	LatestBlock() (uint64, error)
	GetMEVTx(tx string) (*MEVTransaction, error)
	GetMEVBlock(block string) (*MEVBlock, error)
	OldestBlock() uint64
	SaveMEVBLock(block *MEVBlock, txs []*MEVTransaction) error
}

func NewStorage(conn string) (MEVTraceStorage, error) {
	return NewDatabaseService(conn)
}
