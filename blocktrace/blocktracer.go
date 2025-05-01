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
	CALL_TIMEOUT          = 10 * time.Second
	POLLING_INTERVAL      = 6 * time.Second
	LAST_BLOCK_RPC        = "eth_blockNumber"
	TRACE_BLOCK_RPC       = "trace_block"
	BLOCK_BY_HASH_RPC     = "eth_getBlockByHash"
	HEX_PREFIX            = "0x"
	LAST_CONSIDERED_BLOCK = 21_000_000
	FLASHBOTS_COINBASE    = "0xdafea492d9c6733ae3d56b7ed1adb60692c98bc5"
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

func (t *Tracer) Start(ctx context.Context, polling_interval time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(polling_interval):
		}
		t.log.Debug("Polling chain for head block...")
		// first get the latest saved block on the DB
		lastDBBlock := t.storage.LatestBlock()
		// now get the latest block from the chain
		ctx, cancel := context.WithTimeout(context.Background(), CALL_TIMEOUT)
		resp, err := t.rpcClient.Call(ctx, LAST_BLOCK_RPC, nil)
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed rpc call", "endpoint", LAST_BLOCK_RPC, "error", err)
			// no use to do anything at this point
			continue
		}
		cancel()
		lastChainBlockStr, err := resp.GetString()
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed to get string from response", "endpoint", LAST_BLOCK_RPC, "error", err)
		}
		lastChainBlockStr = sanitizeHexString(lastChainBlockStr)
		lastChainBlock, err := strconv.ParseUint(lastChainBlockStr, 16, 64)
		if err != nil {
			// TODO add to error metrics
			t.log.Error("failed to parse string into uint", "endpoint", LAST_BLOCK_RPC, "error", err)
			continue
		}
		t.log.Debug("last chain block", "number", lastChainBlock)

		if lastDBBlock < uint64(lastChainBlock) {
			t.log.Info("Need to catch up with chain", slog.Uint64("latest DB block", lastDBBlock), slog.Uint64("latest chain block", uint64(lastChainBlock)))
			last := lastDBBlock
			for last <= uint64(lastChainBlock) {
				ctx, cancel = context.WithTimeout(context.Background(), CALL_TIMEOUT)
				fetch := fmt.Sprintf("0x%x", last)
				t.log.Debug("Fetching...", slog.String("block", fetch))
				resp, err := t.rpcClient.Call(ctx, TRACE_BLOCK_RPC, fetch)
				defer cancel()
				if err != nil {
					// TODO add to error metrics
					t.log.Error("failed rpc call", "endpoint", TRACE_BLOCK_RPC, "error", err)
					continue
				}
				var traceBlock TraceBlockResponse
				err = resp.GetObject(&traceBlock)
				if err != nil {
					// TODO add to error metrics
					t.log.Error("failed to get block data from response", "endpoint", TRACE_BLOCK_RPC, "error", err)
				}
				t.log.Debug("Fetched traceBlock", slog.Any("block", traceBlock[0].BlockHash))
				blockHash := traceBlock[0].BlockHash
				ctx, cancel = context.WithTimeout(context.Background(), CALL_TIMEOUT)
				actualBlock, err := t.rpcClient.Call(ctx, BLOCK_BY_HASH_RPC, blockHash, false)
				defer cancel()
				if err != nil {
					t.log.Error("failed rpc call", "endpoint", BLOCK_BY_HASH_RPC, "error", err)
					continue
				}
				t.log.Debug("Fetched block by hash")
				var block Block
				err = actualBlock.GetObject(&block)
				if err != nil {
					// TODO add to error metrics
					t.log.Error("failed to get block from response", "endpoint", BLOCK_BY_HASH_RPC, "error", err)
					continue
				}
				isFlashbotMiner := block.Miner == FLASHBOTS_COINBASE
				if isFlashbotMiner {
					t.log.Debug("this block was mined by flashbots", "hash", blockHash)
				}
				t.log.Debug("miner", slog.String("address", block.Miner))
				txs := make([]string, 0)
				total := big.NewInt(0)
				for _, tx := range traceBlock {
					if tx.Action.To == block.Miner {
						t.log.Debug("found tx for coinbase address", "hash", tx.TransactionHash)
						valStr := sanitizeHexString(tx.Action.Value)
						val := new(big.Int)
						val, ok := val.SetString(valStr, 16)
						if !ok {
							t.log.Error("Failed to set the transaction value!", "val", tx.Action.Value)
						}
						total = total.Add(total, val)
						txs = append(txs, tx.TransactionHash)
					}
				}
				if len(txs) > 0 {
					mev_block := &database.MEVBlock{
						BlockNumber:     last,
						BlockHash:       blockHash,
						MEVTransactions: txs,
						Miner:           block.Miner,
						IsFlashbotMiner: isFlashbotMiner,
						TotalMinerValue: total,
					}
					t.log.Debug("saving entry to DB...", "blockNumber", last)
					if err := t.storage.SaveMEVBLock(mev_block); err != nil {
						// TODO: In this case, either retry, or catch up later...
						// e.g. add to some queue or data structure for getting this block again
						// or just retry storing later
						t.log.Error("Failed to save MEV block to database!", "error", err)
					}
				}
				last += 1
				t.log.Debug("getting next block", "last", last)
			}
			t.log.Info("Caught up with chain head")
		} else {
			t.log.Info("DB is in sync with chain head")
		}
	}
}

func sanitizeHexString(s string) string {
	if strings.HasPrefix(s, HEX_PREFIX) {
		return s[2:]
	}
	return s
}
