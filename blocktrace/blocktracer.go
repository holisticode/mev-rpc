package blocktrace

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/flashbots/go-utils/rpcclient"
)

const (
	CALL_TIMEOUT          = 10 * time.Second
	LAST_BLOCK_RPC        = "eth_blockNumber"
	TRACE_BLOCK_RPC       = "trace_block"
	BLOCK_BY_HASH_RPC     = "eth_getBlockByHash"
	HEX_PREFIX            = "0x"
	LAST_CONSIDERED_BLOCK = 21_000_000
	FLASHBOTS_COINBASE    = "0xdafea492d9c6733ae3d56b7ed1adb60692c98bc5"
)

type TraceBlockResponse []BlockData

type BlockData struct {
	Action              Action   `json:"action"`
	BlockHash           string   `json:"blockHash"`
	BlockNumber         uint64   `json:"blockNumber"`
	Result              Result   `json:"result"`
	Subtraces           uint64   `json:"subtraces"`
	TraceAddress        []uint64 `json:"traceAddress"`
	TransactionHash     string   `json:"transactionHash"`
	TransactionPosition uint64   `json:"transactionPosition"`
	Type                string   `json:"type"`
}

type Action struct {
	From     string `json:"from"`
	CallType string `json:"callType"`
	Gas      string `json:"gas"`
	Input    string `json:"input"`
	To       string `json:"to"`
	Value    string `json:"value"`
}

type Result struct {
	GasUsed string `json:"gasUsed"`
	Output  string `json:"output"`
}

type Block struct {
	Number           string   `json:"number"`
	Hash             string   `json:"hash"`
	Transactions     []string `json:"transactions"`
	TotalDifficulty  string   `json:"totalDifficulty"`
	LogsBloom        string   `json:"logsBloom"`
	ReceiptsRoot     string   `json:"receiptsRoot"`
	ExtraData        string   `json:"extraData"`
	BaseFeePerGas    string   `json:"baseFeePerGas"`
	Nonce            string   `json:"nonce"`
	Miner            string   `json:"miner"`
	Difficulty       string   `json:"difficulty"`
	GasLimit         string   `json:"gasLimit"`
	GasUsed          string   `json:"gasUsed"`
	Uncles           []string `json:"uncles"`
	Sha3Uncles       string   `json:"sha3Uncles"`
	Size             string   `json:"size"`
	TransactionsRoot string   `json:"transactionsRoot"`
	StateRoot        string   `json:"stateRoot"`
	MixHash          string   `json:"mixHash"`
	ParentHash       string   `json:"parentHash"`
	Timestamp        string   `json:"timestamp"`
}

type Tracer struct {
	storage MEVTraceStorage
	rpcurl  string
	log     *slog.Logger
}

func NewBlockTracer(rpcurl string, storage MEVTraceStorage, log *slog.Logger) *Tracer {
	return &Tracer{
		storage,
		rpcurl,
		log,
	}
}

func (t *Tracer) Start() {
	// first get the latest saved block on the DB
	lastDBBlock := t.storage.LatestBlock()
	// now get the latest block from the chain
	rpcClient := rpcclient.NewClient(t.rpcurl)
	ctx, cancel := context.WithTimeout(context.Background(), CALL_TIMEOUT)
	resp, err := rpcClient.Call(ctx, LAST_BLOCK_RPC, nil)
	if err != nil {
		// TODO
		panic(err)
	}
	cancel()
	lastChainBlockStr, err := resp.GetString()
	if err != nil {
		// TODO
		panic(err)
	}
	if strings.HasPrefix(lastChainBlockStr, HEX_PREFIX) {
		lastChainBlockStr = lastChainBlockStr[2:]
	}
	lastChainBlock, err := strconv.ParseUint(lastChainBlockStr, 16, 64)
	if err != nil {
		// TODO
		panic(err)
	}
	t.log.Debug("last chain block", "number", lastChainBlock)

	testing := true

	if lastDBBlock < uint64(lastChainBlock) {
		t.log.Info("Need to catch up with chain", slog.Uint64("latest DB block", lastDBBlock), slog.Uint64("latest chain block", uint64(lastChainBlock)))
		last := lastDBBlock
		for last < uint64(lastChainBlock) {
			ctx, cancel = context.WithTimeout(context.Background(), CALL_TIMEOUT)
			fetch := fmt.Sprintf("%x", last)
			fetch = "0x" + lastChainBlockStr
			t.log.Debug("Fetching...", slog.String("block", fetch))
			resp, err := rpcClient.Call(ctx, TRACE_BLOCK_RPC, fetch)
			// TODO: check resp.RPCError too!
			if err != nil {
				panic(err)
			}
			cancel()
			var traceBlock TraceBlockResponse
			err = resp.GetObject(&traceBlock)
			if err != nil {
				panic(err)
			}
			t.log.Debug("Fetched traceBlock", slog.Any("block", traceBlock[0].BlockHash))
			blockHash := traceBlock[0].BlockHash
			ctx, cancel = context.WithTimeout(context.Background(), CALL_TIMEOUT)
			actualBlock, err := rpcClient.Call(ctx, BLOCK_BY_HASH_RPC, blockHash, false)
			if err != nil {
				panic(err)
			}
			t.log.Debug("Fetched block by hash")
			cancel()
			var block Block
			err = actualBlock.GetObject(&block)
			if err != nil {
				panic(err)
			}
			miner := block.Miner
			if miner == FLASHBOTS_COINBASE {
				t.log.Debug("this block was mined by flashbots", "hash", blockHash)
			}
			t.log.Debug("miner", slog.String("address", miner))
			for _, tx := range traceBlock {
				// t.log.Debug("to:", "hash", tx.Action.To)
				if tx.Action.To == miner {
					t.log.Debug("found tx for coinbase address", "hash", tx.TransactionHash)
				}
			}
			last += 1
			if testing {
				break
			}
		}
	}
}
