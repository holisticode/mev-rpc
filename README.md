# MEV Block Tracer

# Overview

This tool queries the ethereum the ethereum chain for block information and scans its transactions for any transaction which have affected the coinbase address of a given block.

## Requirements

* A RPC endpoint for querying the chain (e.g. Alchemy, Quicknode, your own node, etc.)
* A Postgres DB connection for storing relevant information

## Function

`MEV Block Tracer` starts scanning the ethereum chain from block 21_000_000. It queries each block via the `trace_block` RPC call first, retrieving block information.
It then queries the `eth_getBlockByBash` RPC call to get actual block data, including the coinbase address for the block.
Successively it scans each transaction in the block to verify if the transaction changed the coinbase address.
If it does, it saves both block data and some transaction data to the DB.

## JSON-RPC endpoint

The tool offers a JSON-RPC endpoint, which can receive queries about the stored data.
Currently it offers two methods:

* `mev_rpc_tx`
* `mev_rpc_block`

The former allows to get information for a specific transaction, by providing the transaction hash.
The latter returns information for a specific block, including all transactions affecting the coinbase, by providing blocknumber or block hash.

### mev_rpc_tx

For example:

```sh
curl -X POST -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"id","method":"mev_rpc_tx","params":["0x5ec7fe5e57ec42e3de9d30370e39278a8eac3700013b2ec3cb5231fd1a824ac4"]}' http://localhost:8080
```

If the transaction exists in the local DB, it returns that transaction information:

```sh
{"jsonrpc":"2.0","id":"id","result":{"blockNumber":21000001,"txHash":"0x5ec7fe5e57ec42e3de9d30370e39278a8eac3700013b2ec3cb5231fd1a824ac4","from":"0x5ddf30555ee9545c8982626b7e3b6f70e5c2635f","to":"0x4838b106fce9647bdf1e7877bf73ce8b0bad5f97","value":10000000000000}}
```

If the transaction can not be found, it returns an empty response:

```sh
{"jsonrpc":"2.0","id":"id","error":{"code":-32000,"message":"sql: no rows in result set"}}
```

### mev_rpc_block

For example:

```sh
curl -X POST -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":"id","method":"mev_rpc_block","params":["21003051"]}' http://localhost:8080
```

If the block has been store in the local DB, it returns the correspondent information:

```sh
{"jsonrpc":"2.0","id":"id","result":{"blockNumber":21003051,"blockHash":"0x5c0a2b33d14a8e4b25c5aaed9f0f39e76c13eff93cd05c7f1902823b05f05f26","transactions":[{"blockNumber":21003051,"txHash":"0x197431ccae307bf133272e57305aa366b8314b8f89f41f6e6c129e3131677ad1","from":"0x6f1cdbbb4d53d226cf4b917bf768b94acbab6168","to":"0x4838b106fce9647bdf1e7877bf73ce8b0bad5f97","value":359781034660905}],"miner":"0x4838b106fce9647bdf1e7877bf73ce8b0bad5f97","flashbot":false,"totalMinerValue":359781034660905}}
```

If the block is not found, we get an empty response:

```sh
{"jsonrpc":"2.0","id":"id","error":{"code":-32000,"message":"sql: no rows in result set"}}
```

## Continuous operation

The tool first catches up from the latest stored block locally to the last known block on chain, by querying the latest block via the `eth_blockNumber` RPC call.
**This can take a while**

After that, the `MEV Block Tracer` will poll every 6 seconds for a new block and apply its function on this block.

## ReOrgs

`MEV Block Tracer` is tolerant to ReOrgs *in the past* , in the sense that if some block was removed from the chain, then it won't be queried.
However, it is **currently not able** to rollback new blocks added to the chain and then removed due to reorgs.
This is a feature which would have to be added in the future.

# How To Ru

`MEV Block Tracer` requires an `--rpc-endpoint` and a `db-connection-string` command line parameter to operate.

## Building as a binary

To build as a binary, clone the repository, then build the binary, e.g.

```sh
go build -o mev-block-tracer cmd/httpserver/main.go

```

Start your postgres server, then run the tool with (example, use your own `API_KEY`):

```sh
mev-block-tracer --rpc-endpoint https://eth-mainnet.g.alchemy.com/v2/<API_KEY> --db-connection-string "postgres://mev_analytics:$POSTGRES_PASSWORD@localhost:5432/mev_analytics?sslmode=disable" 
```

Per default the DB is created with database name `mev_analytics` , and username `mev_analytics`.

Run `mev-block-tracer --help` for more configuration options.

## docker-compose

Conveniently, there is a docker-compose script which can start everything in a docker environment.

To use it, first build the image for the tool (from project root):

```sh
docker build -t mev-rpc:alpha . -f docker/Dockerfile

```

NOTE: If you want to use a different image tag, you have to change the `docker/docker-compose.yml` file.

You also should provide a `.env` file in the `docker` directory. Example:

```sh
POSTGRES_USER=mev_analytics
POSTGRES_PASSWORD=verysecure
DB_CONNECTION_STRING="postgres://mev_analytics:$POSTGRES_PASSWORD@db:5432/mev_analytics?sslmode=disable"
RPC_ENDPOINT="https://eth-mainnet.g.alchemy.com/v2/$API_KEY"


```

This should allow to start the tool including DB with the familiar `docker-compose up -d` (from the `docker` directory)

# Current limitations

* Failing to save a block currently doesn't have any retry nor queueing mechanism
* New ReOrgs are not handled
* Failures are handled very simplistically. For example, it could be that a call just failed. In that case that block number should be saved and then retried.
* Every block is handled sequentially, this could be severely improved with concurrency

# References

This tool has been built by re-using <https://github.com/flashbots/go-utils> and <https://github.com/flashbots/go-template>
