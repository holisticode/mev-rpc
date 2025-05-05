package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/holisticode/mev-rpc/database"
	"github.com/holisticode/mev-rpc/mocks"
	"github.com/stretchr/testify/require"
)

// TestRPCServerReady() tests server readiness
func TestRPCServerReady(t *testing.T) {
	srv, err := New(&HTTPServerConfig{
		Log: getTestLogger(),
	})
	require.NoError(t, err)
	{ // Check health
		req, err := http.NewRequest(http.MethodGet, "/readyz", nil) //nolint:goconst,nolintlint
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		srv.getRouter().ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}
}

// TestRPCEndpoints() tests our RPC endpoints by mocking the storage,
// "saving" some data and then querying it via the RPC endpoints
func TestRPCEndpoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// mock storage object
	mockStorage := mocks.NewMockMEVTraceStorage(ctrl)
	// create the server
	srv, err := NewJSONRPCServer(&HTTPServerConfig{
		DBService: mockStorage,
		Log:       getTestLogger(),
	})
	require.NoError(t, err)
	// create some fictitious txs
	tx1 := createMEVTx("0x1234")
	tx2 := createMEVTx("0x4321")
	// and the associated block
	block := createMEVBlock()
	block.MEVTransactions = []*database.MEVTransaction{tx1, tx2}
	require.NoError(t, err)
	strNum := strconv.FormatUint(block.BlockNumber, 10)

	// storage mock sequence
	s1 := mockStorage.EXPECT().GetMEVBlock("0x42").Return(nil, nil)
	s2 := mockStorage.EXPECT().GetMEVBlock(strNum).After(s1).Return(block, nil)
	s3 := mockStorage.EXPECT().GetMEVTx("0x4444").After(s2).Return(nil, nil)
	mockStorage.EXPECT().GetMEVTx("0x1234").After(s3).Return(tx1, nil)

	// TODO: The following test sequence could and probably should be refactored
	// (better reuse and grouping)

	{ // 1st request: non-existent block
		jsonReq := `{
            "jsonrpc": "2.0",
            "id": 1,
            "method": "%s",
            "params": ["%s"
            ]
        }`
		jsonReq1 := fmt.Sprintf(jsonReq, RPCModuleByBlock, "0x42")
		reader := bytes.NewReader([]byte(jsonReq1))
		req, err := http.NewRequest(http.MethodPost, "/", reader) //nolint:goconst,nolintlint
		req.Header.Add("Content-Type", "application/json")
		require.NoError(t, err)
		rr := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rr, req)
		var control, empty database.MEVBlock
		var resp map[string]json.RawMessage
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		respReader := bytes.NewReader(resp["result"])
		// when the RPC server returns an error, it justs sets the "result" field to null
		// a way to "catch" this as an error is to unmarshal that, it should get an empty object
		err = json.NewDecoder(respReader).Decode(&control)
		require.NoError(t, err)
		// it seems to still return a StatusOK code, presumably because it is
		// signaling that the call actually succeeded
		require.Equal(t, http.StatusOK, rr.Code)
		// we should get an empty object
		require.Equal(t, &empty, &control)

		// 2nd request: this block exists
		jsonReq2 := fmt.Sprintf(jsonReq, RPCModuleByBlock, strNum)
		reader = bytes.NewReader([]byte(jsonReq2))
		req, err = http.NewRequest(http.MethodPost, "/", reader) //nolint:goconst,nolintlint
		req.Header.Add("Content-Type", "application/json")
		require.NoError(t, err)
		rr = httptest.NewRecorder()
		srv.Handler.ServeHTTP(rr, req)
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		respReader = bytes.NewReader(resp["result"])
		err = json.NewDecoder(respReader).Decode(&control)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rr.Code)
		// therefore unmasrhalling should succeed, and the two objects should be same
		require.Equal(t, block, &control)

		// 3rd request: non-existent tx
		jsonReq3 := fmt.Sprintf(jsonReq, RPCModuleByTX, "0x4444")
		reader = bytes.NewReader([]byte(jsonReq3))
		req, err = http.NewRequest(http.MethodPost, "/", reader) //nolint:goconst,nolintlint
		req.Header.Add("Content-Type", "application/json")
		require.NoError(t, err)
		rr = httptest.NewRecorder()
		srv.Handler.ServeHTTP(rr, req)
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		respReader = bytes.NewReader(resp["result"])
		var emptyTx, controlTx database.MEVTransaction
		err = json.NewDecoder(respReader).Decode(&controlTx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rr.Code)
		// same here, result is set to "null", and unmasrhalling should yield an empty object
		require.Equal(t, &emptyTx, &controlTx)

		// 4th request: this tx exists
		jsonReq4 := fmt.Sprintf(jsonReq, RPCModuleByTX, "0x1234")
		reader = bytes.NewReader([]byte(jsonReq4))
		req, err = http.NewRequest(http.MethodPost, "/", reader) //nolint:goconst,nolintlint
		req.Header.Add("Content-Type", "application/json")
		require.NoError(t, err)
		rr = httptest.NewRecorder()
		srv.Handler.ServeHTTP(rr, req)
		err = json.Unmarshal(rr.Body.Bytes(), &resp)
		require.NoError(t, err)
		respReader = bytes.NewReader(resp["result"])
		err = json.NewDecoder(respReader).Decode(&controlTx)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rr.Code)
		// therefore unmarshalling should succeed, and the two objects should be the same
		require.Equal(t, tx1, &controlTx)
	}
}

func createMEVBlock() *database.MEVBlock {
	return &database.MEVBlock{
		BlockNumber:     21_000_042,
		BlockHash:       "0x1234",
		Miner:           "0x8888",
		IsFlashbotMiner: true,
		TotalMinerValue: big.NewInt(4242),
	}
}

func createMEVTx(txHash string) *database.MEVTransaction {
	return &database.MEVTransaction{
		BlockNumber: 21_000_042,
		TXHash:      txHash,
		From:        "0x1234",
		To:          "0x4321",
		Value:       big.NewInt(42),
	}
}
