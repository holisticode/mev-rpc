// Package database exposes the postgres database
package database

import (
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/holisticode/mev-rpc/database/migrations"
	"github.com/holisticode/mev-rpc/database/vars"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
)

const LAST_CONSIDERED_BLOCK = 21_000_000

type DatabaseService struct {
	DB *sqlx.DB
}

func NewDatabaseService(dsn string) (*DatabaseService, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.DB.SetMaxOpenConns(50)
	db.DB.SetMaxIdleConns(10)
	db.DB.SetConnMaxIdleTime(0)

	// fmt.Println(vars.TableMEVBlocks)
	if os.Getenv("DB_DONT_APPLY_SCHEMA") == "" {
		migrate.SetTable(vars.TableMigrations)
		_, err := migrate.Exec(db.DB, "postgres", migrations.Migrations, migrate.Up)
		if err != nil {
			return nil, err
		}
	}

	dbService := &DatabaseService{DB: db} //nolint:exhaustruct
	err = dbService.prepareNamedQueries()
	return dbService, err
}

func (s *DatabaseService) prepareNamedQueries() (err error) {
	return nil
}

func (s *DatabaseService) Close() error {
	return s.DB.Close()
}

func (s *DatabaseService) LatestBlock() (uint64, error) {
	sel := `SELECT blocknumber from ` + vars.TableMEVBlocks + ` ORDER BY blocknumber DESC LIMIT 1`
	res := s.DB.QueryRow(sel)
	var lastBlock uint64
	if err := res.Scan(&lastBlock); err != nil {
		if err == sql.ErrNoRows {
			return LAST_CONSIDERED_BLOCK, nil
		}
		return LAST_CONSIDERED_BLOCK, err
	}
	if lastBlock < LAST_CONSIDERED_BLOCK {
		lastBlock = LAST_CONSIDERED_BLOCK
	}
	return lastBlock, nil
}

func (s *DatabaseService) OldestBlock() uint64 {
	return 0
}

func (s *DatabaseService) GetMEVBlock(block string) (*MEVBlock, error) {
	searchCol := "blocknumber"
	if strings.HasPrefix(block, "0x") {
		searchCol = "blockhash"
	}
	sel := `SELECT b.*,t.* from ` + vars.TableMEVBlocks + ` b INNER JOIN ` + vars.TableMEVTxs + ` t ON b.id = t.block_id WHERE b.` + searchCol + ` = ($1)`
	rows, err := s.DB.Query(sel, block)
	if err != nil {
		return nil, err
	}

	var txs []*MEVTransaction
	var (
		b_id        uint64
		blocknumber uint64
		blockhash   string
		miner       string
		flashbot    bool
		total       string
	)

	count := 0

	for rows.Next() {
		count++
		var (
			t_id     uint64
			block_id uint64
			blockNum uint64
			hash     string
			from     string
			to       string
			value    string
		)
		if err := rows.Scan(
			&b_id,
			&blocknumber,
			&blockhash,
			&miner,
			&flashbot,
			&total,
			&t_id,
			&block_id,
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

	if count == 0 {
		return nil, sql.ErrNoRows
	}
	tot := new(big.Int)
	tot.SetString(total, 10)
	return &MEVBlock{
		BlockNumber:     blocknumber,
		BlockHash:       blockhash,
		MEVTransactions: txs,
		Miner:           miner,
		IsFlashbotMiner: flashbot,
		TotalMinerValue: tot,
	}, nil
}

func (s *DatabaseService) GetMEVTx(txhash string) (*MEVTransaction, error) {
	sel := `SELECT * from ` + vars.TableMEVTxs + ` WHERE txhash = ($1)`
	row := s.DB.QueryRow(sel, txhash)
	var (
		id       uint64
		block_id uint64
		blockNum uint64
		hash     string
		from     string
		to       string
		value    string
	)
	if err := row.Scan(&id, &block_id, &blockNum, &hash, &from, &to, &value); err != nil {
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

func (s *DatabaseService) SaveMEVBLock(block *MEVBlock, txs []*MEVTransaction) error {
	insertBlock := `INSERT INTO ` + vars.TableMEVBlocks + `(blocknumber, blockhash, miner, flashbot, total) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	insertTxs := `INSERT INTO ` + vars.TableMEVTxs + `(block_id, blocknumber, txhash, src, dest, value) VALUES (:block_id, :blocknumber, :txhash, :src, :dest, :value)`
	value := block.TotalMinerValue.String()
	beginTx, err := s.DB.Beginx()
	if err != nil {
		return fmt.Errorf("failed to initiate begin tx: %v", err)
	}
	defer beginTx.Rollback()

	bRes := beginTx.QueryRowx(insertBlock, block.BlockNumber, block.BlockHash, block.Miner, block.IsFlashbotMiner, value)
	var blockId uint64
	err = bRes.Scan(&blockId)
	if err != nil {
		return fmt.Errorf("failed to get last inserted ID: %v", err)
	}
	txMap := []map[string]interface{}{}

	for _, tx := range txs {
		valStr := tx.Value.String()
		thisTx := map[string]interface{}{
			"block_id":    blockId,
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
		return fmt.Errorf("failed to insert transactions into DB: %v", err)
	}

	if err := beginTx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx to DB:  %v", err)
	}
	return nil
}
