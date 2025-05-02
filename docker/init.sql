CREATE TABLE IF NOT EXISTS mev_analytics (
    id SERIAL PRIMARY KEY,
    blocknumber bigint,
    blockhash text,
    txs text,
    miner text,
    flashbot bool,
    total text 
);
