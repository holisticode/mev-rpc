// Package database exposes the postgres database
package database

import (
	"fmt"
	"os"
	"strings"

	"github.com/holisticode/mev-rpc/database/migrations"
	"github.com/holisticode/mev-rpc/database/vars"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
)

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

func (s *DatabaseService) SomeQuery() (count uint64, err error) {
	query := `SELECT COUNT(*) FROM ` + vars.TableTest + `;`
	row := s.DB.QueryRow(query)
	err = row.Scan(&count)
	return count, err
}

func (s *DatabaseService) LatestBlock() uint64 {
	return 21_000_000
}

func (s *DatabaseService) OldestBlock() uint64 {
	return 0
}

func (s *DatabaseService) SaveMEVBLock(block *MEVBlock) error {
	insert := `INSERT INTO mev_analytics (blocknumber, blockhash, txs, miner, flashbot, total) VALUES ($1, $2, $3, $4, $5, $6 )`
	txs := strings.Join(block.MEVTransactions, ",")
	value := block.TotalMinerValue.String()
	_, err := s.DB.Exec(insert, block.BlockNumber, block.BlockHash, txs, block.Miner, block.IsFlashbotMiner, value)
	if err != nil {
		return fmt.Errorf("failed to insert row in DB: %v", err)
	}
	return nil
}
