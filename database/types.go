package database

import (
	"database/sql"
	"math/big"
	"time"
)

// MEVBlock is the datatype for a block we are going to store in the DB
type MEVBlock struct {
	BlockNumber     uint64            `json:"blockNumber"` //nolint:tagliatelle
	BlockHash       string            `json:"blockHash"`   //nolint:tagliatelle
	MEVTransactions []*MEVTransaction `json:"transactions"`
	Miner           string            `json:"miner"`
	IsFlashbotMiner bool              `json:"flashbot"`
	TotalMinerValue *big.Int          `json:"totalMinerValue"` //nolint:tagliatelle
}

// MEVTransaction is the datatype for a tx we are going to store in the DB
type MEVTransaction struct {
	BlockNumber uint64   `json:"blockNumber"` //nolint:tagliatelle
	TXHash      string   `json:"txHash"`      //nolint:tagliatelle
	From        string   `json:"from"`
	To          string   `json:"to"`
	Value       *big.Int `json:"value"`
}

func NewNullInt64(i int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: i,
		Valid: true,
	}
}

func NewNullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

// NewNullTime returns a sql.NullTime with the given time.Time. If the time is
// the zero value, the NullTime is invalid.
func NewNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  t,
		Valid: t != time.Time{},
	}
}
