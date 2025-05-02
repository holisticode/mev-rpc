// Package vars contains the database variables such as dynamic table names
package vars

import "github.com/holisticode/mev-rpc/common"

var (
	tablePrefix = common.GetEnv("DB_TABLE_PREFIX", "mev")
	tableSuffix = common.GetEnv("DB_TABLE_SUFFIX", "dev")

	TableMigrations = tablePrefix + "_migrations" + tableSuffix
	TableMEVBlocks  = tablePrefix + "_blocks_" + tableSuffix
	TableMEVTxs     = tablePrefix + "_txs_" + tableSuffix
)
