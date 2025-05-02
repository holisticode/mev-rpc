CREATE TABLE IF NOT EXISTS mev_blocks (
    id SERIAL PRIMARY KEY,
    blocknumber bigint,
    blockhash text,
    miner text,
    flashbot bool,
    total text 
);

CREATE TABLE IF NOT EXISTS mev_txs (
  id SERIAL PRIMARY KEY,
  block_id int NOT NULL,
	txhash text,
	src    text, 
	dest   text, 
	value  text,
  CONSTRAINT fk_block FOREIGN KEY(block_id) REFERENCES mev_blocks(id)
);
