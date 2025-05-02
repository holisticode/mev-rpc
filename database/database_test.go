package database

import (
	"database/sql"
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

	db, err := NewDatabaseService(testDBDSN)
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

func Test_GetMEVBlock(t *testing.T) {
	db := resetDatabase(t)
	_, err := db.GetMEVBlock("0x1234")
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = db.GetMEVBlock("1234")
	require.ErrorIs(t, err, sql.ErrNoRows)
	mev_block := createMEVBlock()
	txHash1 := "0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5a5bb50"
	txHash2 := "0xb5c8bd9430b6cc87a0e2fe11aaaaaaaaaaaaaaaaaa4bc8cd032f768fc5a5bb50"
	mev_tx1 := createMEVTx(txHash1)
	mev_tx2 := createMEVTx(txHash2)
	txs := []*MEVTransaction{mev_tx1, mev_tx2}
	mev_block.MEVTransactions = txs
	err = db.SaveMEVBLock(mev_block, txs)
	require.NoError(t, err)
	control, err := db.GetMEVBlock(mev_block.BlockHash)
	require.NoError(t, err)
	require.EqualValues(t, mev_block, control)
}

func Test_GetMEVTx(t *testing.T) {
	db := resetDatabase(t)
	_, err := db.GetMEVTx("0x1234")
	require.ErrorIs(t, err, sql.ErrNoRows)
	txHash := "0xb5c8bd9430b6cc87a0e2fe110ece6bf527fa4f170a4bc8cd032f768fc5a5bb50"
	mev_block := createMEVBlock()
	mev_tx := createMEVTx(txHash)
	err = db.SaveMEVBLock(mev_block, []*MEVTransaction{mev_tx})
	require.NoError(t, err)
	control, err := db.GetMEVTx(txHash)
	require.NoError(t, err)
	require.EqualValues(t, mev_tx, control)
}

func Test_LatestBlock(t *testing.T) {
	db := resetDatabase(t)
	x, err := db.LatestBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(LAST_CONSIDERED_BLOCK), x)
	insertBlock := insertBlockQuery()
	_ = db.DB.QueryRow(insertBlock, 21_000_042, "0x1234", "0x1234", true, 4242)
	x, err = db.LatestBlock()
	require.NoError(t, err)
	require.Equal(t, uint64(21_000_042), x)
}

func insertBlockQuery() string {
	return `INSERT INTO ` + vars.TableMEVBlocks + `(blocknumber, blockhash, miner, flashbot, total) VALUES ($1, $2, $3, $4, $5) RETURNING id`
}

func insertTxQuery() string {
	return `INSERT INTO ` + vars.TableMEVTxs + `(block_id, txhash, src, dest, value) VALUES ($1, $2, $3, $4, $5) RETURNING id`
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
