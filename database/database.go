// Package database exposes the postgres database
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"

	"github.com/holisticode/mev-rpc/database/migrations"
	"github.com/holisticode/mev-rpc/database/vars"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
)

// LastConsideredBlock is the block number from which we start scanning
const LastConsideredBlock = 21_000_000

type DatabaseService struct {
	DB  *sqlx.DB
	log *slog.Logger
}

func NewDatabaseService(dsn string, log *slog.Logger) (*DatabaseService, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(0)

	// fmt.Println(vars.TableMEVBlocks)
	if os.Getenv("DB_DONT_APPLY_SCHEMA") == "" {
		migrate.SetTable(vars.TableMigrations)
		_, err := migrate.Exec(db.DB, "postgres", migrations.Migrations, migrate.Up)
		if err != nil {
			return nil, err
		}
	}

	dbService := &DatabaseService{DB: db, log: log} //nolint:exhaustruct
	err = dbService.prepareNamedQueries()
	return dbService, err
}

func (s *DatabaseService) Close() error {
	return s.DB.Close()
}

// LatestBlock returns the latest block we stored in our DB
func (s *DatabaseService) LatestBlock() (uint64, error) {
	sel := `SELECT blocknumber from ` + vars.TableMEVBlocks + ` ORDER BY blocknumber DESC LIMIT 1`
	res := s.DB.QueryRow(sel)
	var lastBlock uint64
	if err := res.Scan(&lastBlock); err != nil {
		if err == sql.ErrNoRows {
			return LastConsideredBlock, nil
		}
		return LastConsideredBlock, err
	}
	if lastBlock < LastConsideredBlock {
		lastBlock = LastConsideredBlock
	}
	return lastBlock, nil
}

// OldestBlock is currently unused
func (s *DatabaseService) OldestBlock() uint64 {
	return 0
}

// GetMEVBlock returns a block by its number OR its hash
// Returns an error if the block can not be found
func (s *DatabaseService) GetMEVBlock(block string) (*MEVBlock, error) {
	searchCol := "blocknumber"
	if strings.HasPrefix(block, "0x") {
		searchCol = "blockhash"
	}
	// This SQL code will return a row for each tx associated to the block, which means...
	sel := `SELECT b.*,t.* from ` + vars.TableMEVBlocks + ` b INNER JOIN ` + vars.TableMEVTxs + ` t ON b.id = t.block_id WHERE b.` + searchCol + ` = ($1)`
	rows, err := s.DB.Query(sel, block)
	if err != nil {
		return nil, err
	}

	var txs []*MEVTransaction
	var (
		bID         uint64
		blocknumber uint64
		blockhash   string
		miner       string
		flashbot    bool
		total       string
	)

	count := 0

	// TODO: ...that this can and probably should be refactored
	// Because the block data is being repeated on every row scan
	for rows.Next() {
		count++
		var (
			tID      uint64
			blockID  uint64
			blockNum uint64
			hash     string
			from     string
			to       string
			value    string
		)
		if err := rows.Scan(
			&bID,
			&blocknumber,
			&blockhash,
			&miner,
			&flashbot,
			&total,
			&tID,
			&blockID,
			&blockNum,
			&hash,
			&from,
			&to,
			&value); err != nil {
			return nil, err
		}

		val := new(big.Int)
		val.SetString(value, 10)
		tx := &MEVTransaction{
			BlockNumber: blockNum,
			TXHash:      hash,
			From:        from,
			To:          to,
			Value:       val,
		}
		txs = append(txs, tx)
	}

	// if now txs could be found then there's no block either
	if count == 0 {
		return nil, sql.ErrNoRows
	}
	tot := new(big.Int)
	tot.SetString(total, 10)
	// return the block with its transactions
	return &MEVBlock{
		BlockNumber:     blocknumber,
		BlockHash:       blockhash,
		MEVTransactions: txs,
		Miner:           miner,
		IsFlashbotMiner: flashbot,
		TotalMinerValue: tot,
	}, nil
}

// GetMEVTx returns a single tx by its hash
// If it can't find the tx, it returns an error
func (s *DatabaseService) GetMEVTx(txhash string) (*MEVTransaction, error) {
	sel := `SELECT * from ` + vars.TableMEVTxs + ` WHERE txhash = ($1)`
	row := s.DB.QueryRow(sel, txhash)
	var (
		id       uint64
		blockID  uint64
		blockNum uint64
		hash     string
		from     string
		to       string
		value    string
	)
	if err := row.Scan(&id, &blockID, &blockNum, &hash, &from, &to, &value); err != nil {
		return nil, err
	}
	val := new(big.Int)
	val.SetString(value, 10)
	return &MEVTransaction{
		BlockNumber: blockNum,
		TXHash:      hash,
		From:        from,
		To:          to,
		Value:       val,
	}, nil
}

// SaveMEVBLock saves the block and its transactions to disk in a one to many relationship
func (s *DatabaseService) SaveMEVBLock(block *MEVBlock, txs []*MEVTransaction) error {
	insertBlock := `INSERT INTO ` + vars.TableMEVBlocks + `(blocknumber, blockhash, miner, flashbot, total) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	insertTxs := `INSERT INTO ` + vars.TableMEVTxs + `(block_id, blocknumber, txhash, src, dest, value) VALUES (:block_id, :blocknumber, :txhash, :src, :dest, :value)`
	value := block.TotalMinerValue.String()
	beginTx, err := s.DB.Beginx()
	if err != nil {
		return fmt.Errorf("failed to initiate begin tx: %w", err)
	}
	defer func() {
		err := beginTx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			s.log.Error("Failed to rollback TX!", "error", err)
		}
	}()

	bRes := beginTx.QueryRowx(insertBlock, block.BlockNumber, block.BlockHash, block.Miner, block.IsFlashbotMiner, value)
	var blockID uint64
	err = bRes.Scan(&blockID)
	if err != nil {
		return fmt.Errorf("failed to get last inserted ID: %w", err)
	}
	txMap := []map[string]interface{}{}

	for _, tx := range txs {
		valStr := tx.Value.String()
		thisTx := map[string]interface{}{
			"block_id":    blockID,
			"blocknumber": block.BlockNumber,
			"txhash":      tx.TXHash,
			"src":         tx.From,
			"dest":        tx.To,
			"value":       valStr,
		}
		txMap = append(txMap, thisTx)
	}

	_, err = beginTx.NamedExec(insertTxs, txMap)
	if err != nil {
		return fmt.Errorf("failed to insert transactions into DB: %w", err)
	}

	if err := beginTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx to DB:  %w", err)
	}
	s.log.Debug("block saved successfully", "block", block.BlockNumber)
	return nil
}

func (s *DatabaseService) prepareNamedQueries() (err error) {
	return nil
}
