// Package blocktrace implements the core of the work test.
// It contains logic to query an ethereum node for the required
// RPC endpoints and process the respnses to build up a
// database of transactions and blocks which coinbase address
// were affected by MEV relevant transactions
package blocktrace

import (
	"context"
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

type Tracer struct {
	storage   database.MEVTraceStorage
	rpcClient rpcclient.RPCClient
	log       *slog.Logger
}

func NewBlockTracer(rpcClient rpcclient.RPCClient, storage database.MEVTraceStorage, log *slog.Logger) *Tracer {
	return &Tracer{
		storage,
		rpcClient,
		log,
	}
}

func (t *Tracer) Start(ctx context.Context, pollingInterval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollingInterval):
		}
		t.log.Debug("Polling chain for head block...")
		// first get the latest saved block on the DB
		lastDBBlock, err := t.storage.LatestBlock()
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed rpc call", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point
			continue
		}
		// now get the latest block from the chain
		ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
		resp, err := t.rpcClient.Call(ctx, LastBlockRPC, nil)
		defer cancel()
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed rpc call", "endpoint", LastBlockRPC, "error", err)
			// no use to do anything at this point
			continue
		}
		lastChainBlockStr, err := resp.GetString()
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed to get string from response", "endpoint", LastBlockRPC, "error", err)
			continue
		}
		lastChainBlockStr = sanitizeHexString(lastChainBlockStr)
		lastChainBlock, err := strconv.ParseUint(lastChainBlockStr, 16, 64)
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed to parse string into uint", "endpoint", LastBlockRPC, "error", err)
			continue
		}
		t.log.Debug("last chain block", "number", lastChainBlock)

		if lastDBBlock < lastChainBlock {
			t.catchUp(lastDBBlock, lastChainBlock)
		} else {
			t.log.Info("DB is in sync with chain head")
		}
	}
}

func sanitizeHexString(s string) string {
	if strings.HasPrefix(s, HexPrefix) {
		return s[2:]
	}
	return s
}

func (t *Tracer) catchUp(lastDBBlock, lastChainBlock uint64) {
	t.log.Info("Need to catch up with chain",
		slog.Uint64("latest DB block", lastDBBlock),
		slog.Uint64("latest chain block", lastChainBlock))
	last := lastDBBlock
	for last <= lastChainBlock {
		tB, err := t.traceBlock(last)
		if err != nil {
			continue
		}
		traceBlock := *tB
		blockHash := traceBlock[0].BlockHash
		ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
		defer cancel()
		actualBlock, err := t.rpcClient.Call(ctx, BlockByHashRPC, blockHash, false)
		if err != nil {
			t.log.Error("failed rpc call", "endpoint", BlockByHashRPC, "error", err)
			continue
		}
		t.log.Debug("Fetched block by hash")
		var block Block
		err = actualBlock.GetObject(&block)
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed to get block from response", "endpoint", BlockByHashRPC, "error", err)
			continue
		}

		t.handleTxs(tB, &block, blockHash, last)

		last += 1
		t.log.Debug("getting next block", "last", last)
	}
	t.log.Info("Caught up with chain head")
}

func (t *Tracer) handleTxs(
	traceBlock *TraceBlockResponse,
	block *Block,
	blockHash string,
	blockNum uint64,
) {
	isFlashbotMiner := block.Miner == FlashbotsCoinbase
	if isFlashbotMiner {
		t.log.Debug("this block was mined by flashbots", "hash", blockHash)
	}
	t.log.Debug("miner", slog.String("address", block.Miner))
	txs := make([]*database.MEVTransaction, 0)
	total := big.NewInt(0)
	for _, tx := range *traceBlock {
		if tx.Action.To == block.Miner {
			t.log.Debug("found tx for coinbase address", "hash", tx.TransactionHash)
			// TODO: maybe we don't need to convert the value to big.Int,
			// as we are going to store the value as string in the database again?
			valStr := sanitizeHexString(tx.Action.Value)
			val := new(big.Int)
			val, ok := val.SetString(valStr, 16)
			if !ok {
				t.log.Error("Failed to set the transaction value!", "val", tx.Action.Value)
			}
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
	if len(txs) > 0 {
		mevBlock := &database.MEVBlock{
			BlockNumber:     blockNum,
			BlockHash:       blockHash,
			Miner:           block.Miner,
			IsFlashbotMiner: isFlashbotMiner,
			TotalMinerValue: total,
		}
		t.log.Debug("saving block and txs to DB...", "blockNumber", blockNum)
		if err := t.storage.SaveMEVBLock(mevBlock, txs); err != nil {
			// TODO: In this case, either retry, or catch up later...
			// e.g. add to some queue or data structure for getting this block again
			// or just retry storing later
			t.log.Error("Failed to save MEV block to database!", "error", err)
		}
	}
}

func (t *Tracer) traceBlock(block uint64) (*TraceBlockResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), CallTimeout)
	defer cancel()
	fetch := fmt.Sprintf("0x%x", block)
	t.log.Debug("Fetching...", slog.String("block", fetch))
	resp, err := t.rpcClient.Call(ctx, TraceBlockRPC, fetch)
	if err != nil {
		// TODO add to error metrics
		t.log.Error("failed rpc call", "endpoint", TraceBlockRPC, "error", err)
	}
	var traceBlock *TraceBlockResponse
	err = resp.GetObject(&traceBlock)
	if err != nil {
		// TODO add to error metrics
		t.log.Error("failed to get block data from response", "endpoint", TraceBlockRPC, "error", err)
		return nil, err
	}
	t.log.Debug("Fetched traceBlock", slog.Any("block", (*traceBlock)[0].BlockHash))
	return traceBlock, nil
}
