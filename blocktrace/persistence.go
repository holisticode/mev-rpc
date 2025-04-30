package blocktrace

type MEVTraceStorage interface {
	LatestBlock() uint64
	OldestBlock() uint64
}

type PGTraceStorage struct {
	conn string
}

func (s *PGTraceStorage) LatestBlock() uint64 {
	return 0
}

func (s *PGTraceStorage) OldestBlock() uint64 {
	return 0
}

func NewStorage(conn string) MEVTraceStorage {
	return &PGTraceStorage{
		conn,
	}
}
