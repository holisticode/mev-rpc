package blocktrace

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
