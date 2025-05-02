package migrations

import (
	"github.com/holisticode/mev-rpc/database/vars"
	migrate "github.com/rubenv/sql-migrate"
)

var Migration001InitDatabase = &migrate.Migration{
	Id: "001-init-database",
	Up: []string{`
		CREATE TABLE IF NOT EXISTS ` + vars.TableMEVBlocks + ` (
		    id SERIAL PRIMARY KEY,
    		blocknumber bigint,
    		blockhash text,
    		miner text,
    		flashbot bool,
    		total text 
		);
		CREATE TABLE IF NOT EXISTS ` + vars.TableMEVTxs + ` (
	      id SERIAL PRIMARY KEY,
				block_id int NOT NULL,
				blocknumber bigint,
				txhash text,
				src    text, 
				dest   text, 
				value  text,
				CONSTRAINT fk_block FOREIGN KEY(block_id) REFERENCES ` + vars.TableMEVBlocks + `(id)
		);
	`},
	Down: []string{`
		DROP TABLE IF EXISTS ` + vars.TableMEVBlocks + `;
		DROP TABLE IF EXISTS ` + vars.TableMEVTxs + `;
	`},
	DisableTransactionUp:   false,
	DisableTransactionDown: false,
}
