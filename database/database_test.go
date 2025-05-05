package database

import (
	"database/sql"
	"log/slog"
	"math/big"
	"os"
	"testing"

	"github.com/holisticode/mev-rpc/common"
	"github.com/holisticode/mev-rpc/database/migrations"
	"github.com/holisticode/mev-rpc/database/vars"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

var (
	runDBTests = os.Getenv("RUN_DB_TESTS") == "1" //|| true
	testDBDSN  = common.GetEnv("TEST_DB_DSN", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
)

func resetDatabase(t *testing.T) *DatabaseService {
	t.Helper()
	if !runDBTests {
		t.Skip("Skipping database tests")
	}

	// This actually doesn't work as the vars are global and initialied earlier
	t.Setenv("DB_TABLE_SUFFIX", "test")

	// Wipe test database
	_db, err := sqlx.Connect("postgres", testDBDSN)
	require.NoError(t, err)
	_, err = _db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	require.NoError(t, err)

	db, err := NewDatabaseService(testDBDSN, getTestLogger())
	require.NoError(t, err)
	return db
}

func TestMigrations(t *testing.T) {
	db := resetDatabase(t)
	query := `SELECT COUNT(*) FROM ` + vars.TableMigrations + `;`
	rowCount := 0
	err := db.DB.QueryRow(query).Scan(&rowCount)
	require.NoError(t, err)
	require.Len(t, migrations.Migrations.Migrations, rowCount)
}

// Test_GetMEVBlock() tests that we can save and get a block from the DB
func Test_GetMEVBlock(t *testing.T) {
	db := resetDatabase(t)
	// there should be no block yet (by hash)
	_, err := db.GetMEVBlock("0x1234")
	require.ErrorIs(t, err, sql.ErrNoRows)

	// also by number
	_, err = db.GetMEVBlock("1234")
	require.ErrorIs(t, err, sql.ErrNoRows)

	// let's create a dummy block
	mevBlock := createMEVBlock()
	// and create some txs which would be in the block
	txHash1 := "0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5a5bb50"
	txHash2 := "0xb5c8bd9430b6cc87a0e2fe11aaaaaaaaaaaaaaaaaa4bc8cd032f768fc5a5bb50"
	mevTx1 := createMEVTx(txHash1)
	mevTx2 := createMEVTx(txHash2)
	txs := []*MEVTransaction{mevTx1, mevTx2}
	mevBlock.MEVTransactions = txs
	// save the block to DB
	err = db.SaveMEVBLock(mevBlock, txs)
	require.NoError(t, err)
	// now get the same block again
	control, err := db.GetMEVBlock(mevBlock.BlockHash)
	require.NoError(t, err)
	// should be the same
	require.Equal(t, mevBlock, control)
}

// Test_GetMEVTx() tests that we can save and get a single tx
func Test_GetMEVTx(t *testing.T) {
	db := resetDatabase(t)
	// there should be no tx yet
	_, err := db.GetMEVTx("0x1234")
	require.ErrorIs(t, err, sql.ErrNoRows)

	// let's create a dummy tx
	txHash := "0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5a5bb50"
	// a tx is always stored with an associated block
	mevBlock := createMEVBlock()
	mevTx := createMEVTx(txHash)
	err = db.SaveMEVBLock(mevBlock, []*MEVTransaction{mevTx})
	require.NoError(t, err)
	// but we can still query for a single tx only (for the RPC endpoint)
	control, err := db.GetMEVTx(txHash)
	require.NoError(t, err)
	// should be the same
	require.Equal(t, mevTx, control)
}

// Test_LatestBlock() tests that LatestBlock returns the expected block number
func Test_LatestBlock(t *testing.T) {
	db := resetDatabase(t)
	x, err := db.LatestBlock()
	require.NoError(t, err)
	// we should start at the minimum number configured
	require.Equal(t, uint64(LastConsideredBlock), x)
	// insert some block
	insertBlock := insertBlockQuery()
	// get the block again
	_ = db.DB.QueryRow(insertBlock, 21_000_042, "0x1234", "0x1234", true, 4242)
	x, err = db.LatestBlock()
	require.NoError(t, err)
	// latest block should be from the block
	require.Equal(t, uint64(21_000_042), x)
}

func insertBlockQuery() string {
	return `INSERT INTO ` + vars.TableMEVBlocks + `(blocknumber, blockhash, miner, flashbot, total) VALUES ($1, $2, $3, $4, $5) RETURNING id`
}

func createMEVBlock() *MEVBlock {
	return &MEVBlock{
		BlockNumber:     21_000_042,
		BlockHash:       "0x1234",
		Miner:           "0x8888",
		IsFlashbotMiner: true,
		TotalMinerValue: big.NewInt(4242),
	}
}

func createMEVTx(txHash string) *MEVTransaction {
	return &MEVTransaction{
		BlockNumber: 21_000_042,
		TXHash:      txHash,
		From:        "0x1234",
		To:          "0x4321",
		Value:       big.NewInt(42),
	}
}

func getTestLogger() *slog.Logger {
	return common.SetupLogger(&common.LoggingOpts{
		Debug:   true,
		JSON:    false,
		Service: "test",
		Version: "test",
	})
}
