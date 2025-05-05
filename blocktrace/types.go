package blocktrace

// This file contains objects used to marshal/unmarshal to/from JSON

// TraceBlockResponse is the wrapper for the trace_block response
type TraceBlockResponse []BlockData

// BlockData is the actual block info in trace_block
type BlockData struct {
	Action              Action   `json:"action"`
	BlockHash           string   `json:"blockHash"`   //nolint:tagliatelle
	BlockNumber         uint64   `json:"blockNumber"` //nolint:tagliatelle
	Result              Result   `json:"result"`
	Subtraces           uint64   `json:"subtraces"`
	TraceAddress        []uint64 `json:"traceAddress"`        //nolint:tagliatelle
	TransactionHash     string   `json:"transactionHash"`     //nolint:tagliatelle
	TransactionPosition uint64   `json:"transactionPosition"` //nolint:tagliatelle
	Type                string   `json:"type"`
}

// Action contains tx data in blocks
type Action struct {
	From     string `json:"from"`
	CallType string `json:"callType"` //nolint:tagliatelle
	Gas      string `json:"gas"`
	Input    string `json:"input"`
	To       string `json:"to"`
	Value    string `json:"value"`
}

// Result is from the upper level BlockData
type Result struct {
	GasUsed string `json:"gasUsed"` //nolint:tagliatelle
	Output  string `json:"output"`
}

// Block is the representation of the eth_getBlockByHash RPC response
type Block struct {
	Number           string   `json:"number"`
	Hash             string   `json:"hash"`
	Transactions     []string `json:"transactions"`
	TotalDifficulty  string   `json:"totalDifficulty"` //nolint:tagliatelle
	LogsBloom        string   `json:"logsBloom"`       //nolint:tagliatelle
	ReceiptsRoot     string   `json:"receiptsRoot"`    //nolint:tagliatelle
	ExtraData        string   `json:"extraData"`       //nolint:tagliatelle
	BaseFeePerGas    string   `json:"baseFeePerGas"`   //nolint:tagliatelle
	Nonce            string   `json:"nonce"`
	Miner            string   `json:"miner"`
	Difficulty       string   `json:"difficulty"`
	GasLimit         string   `json:"gasLimit"` //nolint:tagliatelle
	GasUsed          string   `json:"gasUsed"`  //nolint:tagliatelle
	Uncles           []string `json:"uncles"`
	Sha3Uncles       string   `json:"sha3Uncles"` //nolint:tagliatelle
	Size             string   `json:"size"`
	TransactionsRoot string   `json:"transactionsRoot"` //nolint:tagliatelle
	StateRoot        string   `json:"stateRoot"`        //nolint:tagliatelle
	MixHash          string   `json:"mixHash"`          //nolint:tagliatelle
	ParentHash       string   `json:"parentHash"`       //nolint:tagliatelle
	Timestamp        string   `json:"timestamp"`
}
