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

func TestRPCEndpoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockStorage := mocks.NewMockMEVTraceStorage(ctrl)
	srv, err := NewJSONRPCServer(&HTTPServerConfig{
		DBService: mockStorage,
		Log:       getTestLogger(),
	})
	require.NoError(t, err)
	tx1 := createMEVTx("0x1234")
	tx2 := createMEVTx("0x4321")
	block := createMEVBlock()
	block.MEVTransactions = []*database.MEVTransaction{tx1, tx2}
	require.NoError(t, err)
	strNum := strconv.FormatUint(block.BlockNumber, 10)
	s1 := mockStorage.EXPECT().GetMEVBlock("0x42").Return(nil, nil)
	s2 := mockStorage.EXPECT().GetMEVBlock(strNum).After(s1).Return(block, nil)
	s3 := mockStorage.EXPECT().GetMEVTx("0x4444").After(s2).Return(nil, nil)
	mockStorage.EXPECT().GetMEVTx("0x1234").After(s3).Return(tx1, nil)
	{ // 1st request
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
		err = json.NewDecoder(respReader).Decode(&control)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rr.Code)
		require.Equal(t, &empty, &control)

		// 2nd request
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
		require.Equal(t, block, &control)

		// 4th request
		jsonReq4 := fmt.Sprintf(jsonReq, RPCModuleByTX, "0x4444")
		reader = bytes.NewReader([]byte(jsonReq4))
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
		require.Equal(t, &emptyTx, &controlTx)

		// 3rd request
		jsonReq3 := fmt.Sprintf(jsonReq, RPCModuleByTX, "0x1234")
		reader = bytes.NewReader([]byte(jsonReq3))
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
