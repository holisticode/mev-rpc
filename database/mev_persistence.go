package database

type MEVTraceStorage interface {
	LatestBlock() uint64
	OldestBlock() uint64
	SaveMEVBLock(block *MEVBlock) error
}

func NewStorage(conn string) (MEVTraceStorage, error) {
	return NewDatabaseService(conn)
}
