package blocktrace

import "github.com/flashbots/go-template/database"

type MEVTraceStorage interface {
	LatestBlock() uint64
	OldestBlock() uint64
}

func NewStorage(conn string) (MEVTraceStorage, error) {
	return database.NewDatabaseService(conn)
}
