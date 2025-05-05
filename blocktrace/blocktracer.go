// Package blocktrace implements the core of the work test.
// It contains logic to query an ethereum node for the required
// RPC endpoints and process the respnses to build up a
// database of transactions and blocks which coinbase address
// were affected by MEV relevant transactions
package blocktrace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/flashbots/go-utils/rpcclient"
	"github.com/holisticode/mev-rpc/database"
)

const (
	CallTimeout       = 10 * time.Second
	PollingInterval   = 6 * time.Second
	LastBlockRPC      = "eth_blockNumber"
	TraceBlockRPC     = "trace_block"
	BlockByHashRPC    = "eth_getBlockByHash"
	HexPrefix         = "0x"
	FlashbotsCoinbase = "0xdafea492d9c6733ae3d56b7ed1adb60692c98bc5"
)

var ErrEmptyBlock = errors.New("empty block")

// Tracer is the main object used to query the chain
type Tracer struct {
	storage   database.MEVTraceStorage
	rpcClient rpcclient.RPCClient
	log       *slog.Logger
}

// NewBlockTracer creates a new tracer.
// Params:
// * rpcClient for querying nodes (can be mocked)
// * storage interface object for storying and querying the data we're interested in (can be mocked)
// * log logger object
func NewBlockTracer(rpcClient rpcclient.RPCClient, storage database.MEVTraceStorage, log *slog.Logger) *Tracer {
	return &Tracer{
		storage,
		rpcClient,
		log,
	}
}

// Start starts to query the chain and retrieve data
// It assumes to be started in a go routine,
// therefore it does not return an error in error conditions.
// The current strategy is to continue looping: error causes could be of
// different nature.
// For example, a non-exisiting block number (reorg?) could have been queried,
// which doesn't exist, in which case, the next could well be succeeding again.
//
// params
// * ctx a context (mainly for canceling the loop)
// * pollingInterval duration (allows to pass custom interval for quicker testing)
// NOTE: The fetching of individual blocks could be parallelized, improving performance
func (t *Tracer) Start(ctx context.Context, pollingInterval time.Duration) {
	// loop endlessly...
	for {
		select {
		// ...unless canceled
		case <-ctx.Done():
			return
		// we wait some time after every loop
		case <-time.After(pollingInterval):
		}
		t.log.Debug("Polling chain for head block...")
		// first get the latest saved block on the DB
		lastDBBlock, err := t.storage.LatestBlock()
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed rpc call", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point
			// TODO:maybe an error counter; after a threshold stop or panic server
			continue
		}
		// now get the latest block from the chain
		ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
		resp, err := t.rpcClient.Call(ctx, LastBlockRPC, nil)
		defer cancel()
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed rpc call", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point
			// TODO:maybe an error counter; after a threshold stop or panic server
			continue
		}
		lastChainBlockStr, err := resp.GetString()
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed to get string from response", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point; however, this error should maybe be handled better:
			// we got data but couldn't interpret it
			continue
		}
		lastChainBlockStr = sanitizeHexString(lastChainBlockStr)
		lastChainBlock, err := strconv.ParseUint(lastChainBlockStr, 16, 64)
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed to parse string into uint", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point; however, this error should maybe be handled better:
			// we got data but couldn't interpret it
			continue
		}
		t.log.Debug("last chain block", "number", lastChainBlock)

		if lastDBBlock < lastChainBlock {
			// our database has a lower chain block number than the last number on chain
			t.catchUp(lastDBBlock, lastChainBlock)
		} else {
			t.log.Info("DB is in sync with chain head")
		}
	}
}

// sanitizeHexString() removes 0x if needed from a hex string
func sanitizeHexString(s string) string {
	if strings.HasPrefix(s, HexPrefix) {
		return s[2:]
	}
	return s
}

// catchUp runs a loop to catch up our database with the latest block on chain.
// It loops from the last saved block number until the last known block on chain.
// It ignores errors for non-existing blocks in between
func (t *Tracer) catchUp(lastDBBlock, lastChainBlock uint64) {
	t.log.Info("Need to catch up with chain",
		slog.Uint64("latest DB block", lastDBBlock),
		slog.Uint64("latest chain block", lastChainBlock))

	last := lastDBBlock
	// iterate from our last DB block until the latest known on chain
	for last <= lastChainBlock {
		// get data from the trace_block RPC
		tB, err := t.traceBlock(last)
		if err != nil {
			// TODO: add to error metrics
			// ignore error
			last += 1
			continue
		}
		traceBlock := *tB
		blockHash := traceBlock[0].BlockHash
		ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
		defer cancel()

		// get the data from the actual block from the eth_getBlockByHash RPC
		actualBlock, err := t.rpcClient.Call(ctx, BlockByHashRPC, blockHash, false)
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed rpc call", "endpoint", BlockByHashRPC, "error", err)
			// ignore, however this type of error should be handled better, as we got data but couldn't interpret it
			last += 1
			continue
		}
		t.log.Debug("Fetched block by hash")
		var block Block
		err = actualBlock.GetObject(&block)
		if err != nil {
			// TODO: add to error metrics
			t.log.Error("failed to get block from response", "endpoint", BlockByHashRPC, "error", err)
			// ignore, however this type of error should be handled better, as we got data but couldn't interpret it
			last += 1
			continue
		}

		// we got the data for the block; extract tx data from it
		t.handleTxs(tB, &block, blockHash, last)

		// handle next block
		last += 1
		t.log.Debug("getting next block", "last", last)
	}
	t.log.Info("Caught up with chain head")
}

// handleTxs extracts the data we are interested in from a block,
// and stores it into the DB
func (t *Tracer) handleTxs(
	traceBlock *TraceBlockResponse,
	block *Block,
	blockHash string,
	blockNum uint64,
) {
	// we might be interested to know if the coinbase address was a flashbot one
	isFlashbotMiner := block.Miner == FlashbotsCoinbase
	if isFlashbotMiner {
		t.log.Debug("this block was mined by flashbots", "hash", blockHash)
	}
	t.log.Debug("miner", slog.String("address", block.Miner))

	txs := make([]*database.MEVTransaction, 0)
	total := big.NewInt(0)

	// iterate all txs of the block
	for _, tx := range *traceBlock {
		// we are interested in txs which destination address is the block's coinbase address
		if tx.Action.To == block.Miner {
			t.log.Debug("found tx for coinbase address", "hash", tx.TransactionHash)
			// TODO: maybe we don't need to convert the value to big.Int,
			// as we are going to store the value as string in the database again?
			// (however, it's good to use a go type inside the go space)
			valStr := sanitizeHexString(tx.Action.Value)
			val := new(big.Int)
			val, ok := val.SetString(valStr, 16)
			if !ok {
				// TODO: add to log metrics
				// this should actually never happen
				t.log.Error("Failed to set the transaction value!", "val", tx.Action.Value)
				continue
			}
			// create an object to store the tx
			mtx := &database.MEVTransaction{
				TXHash:      tx.TransactionHash,
				From:        tx.Action.From,
				To:          tx.Action.To,
				Value:       val,
				BlockNumber: blockNum,
			}
			total = total.Add(total, val)
			txs = append(txs, mtx)
		}
	}
	// only if we had any relevant txs at all...
	if len(txs) > 0 {
		// ...we create a block representation
		mevBlock := &database.MEVBlock{
			BlockNumber:     blockNum,
			BlockHash:       blockHash,
			Miner:           block.Miner,
			IsFlashbotMiner: isFlashbotMiner,
			TotalMinerValue: total,
		}
		t.log.Debug("saving block and txs to DB...", "blockNumber", blockNum)
		// ...and try to save it
		if err := t.storage.SaveMEVBLock(mevBlock, txs); err != nil {
			// TODO: In this case, either retry, or catch up later...
			// e.g. add to some queue or data structure for getting this block again
			// or just retry storing later
			t.log.Error("Failed to save MEV block to database!", "error", err)
		} else {
			t.log.Info("Saved MEV block to database", "block", blockNum)
		}
	}
}

// traceBlock executes the trace_block RPC call
func (t *Tracer) traceBlock(block uint64) (*TraceBlockResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	// number representation of the block we're going to fetch
	fetch := fmt.Sprintf("0x%x", block)
	t.log.Debug("Fetching...", slog.String("block", fetch))
	resp, err := t.rpcClient.Call(ctx, TraceBlockRPC, fetch)
	if err != nil {
		// TODO: add to error metrics
		t.log.Error("failed rpc call", "endpoint", TraceBlockRPC, "error", err)
		return nil, err
	}
	var traceBlock *TraceBlockResponse
	if err = resp.GetObject(&traceBlock); err != nil {
		// TODO: add to error metrics
		t.log.Error("failed to get block data from response", "endpoint", TraceBlockRPC, "error", err)
		return nil, err
	}
	// TODO: Weird, I got a number of such empty blocks while testing,
	// which don't seem to be empty on alchemy...
	if len(*traceBlock) == 0 {
		t.log.Error("trace_block returned empty block", "block", block)
		return nil, ErrEmptyBlock
	}
	t.log.Debug("Fetched traceBlock", slog.Any("block", (*traceBlock)[0].BlockHash))
	return traceBlock, nil
}
