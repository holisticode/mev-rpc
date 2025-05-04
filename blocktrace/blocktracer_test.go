package blocktrace

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flashbots/go-utils/rpcclient"
	"github.com/golang/mock/gomock"
	"github.com/holisticode/mev-rpc/common"
	"github.com/holisticode/mev-rpc/mocks"
	"github.com/stretchr/testify/require"
)

func TestBlockTrace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())
	// Create a mock instance
	mockStorage := mocks.NewMockMEVTraceStorage(ctrl)
	s1 := mockStorage.EXPECT().LatestBlock().Return(uint64(22391064), nil)
	s2 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s1).Return(nil)
	s3 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s2).Return(nil)
	s4 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s3).Return(nil)
	mockStorage.EXPECT().LatestBlock().After(s4).Return(uint64(22391066), nil).Do(cancel)

	// Call the function under test, using the mock as a
	mockRPCClient := mocks.NewMockRPCClient(ctrl)
	jsonHash := getJSON(t, "./testdata/block_number.json")
	r1 := mockRPCClient.EXPECT().Call(gomock.Any(), LastBlockRPC, nil).Return(jsonHash, nil)
	jsonTrace := getJSON(t, "./testdata/trace_block.json")
	r2 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r1).Return(jsonTrace, nil)
	jsonBlock := getJSON(t, "./testdata/block_hash.json")
	r3 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r2).Return(jsonBlock, nil)
	r4 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r3).Return(jsonTrace, nil)
	r5 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r4).Return(jsonBlock, nil)
	r6 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r5).Return(jsonTrace, nil)
	r7 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r6).Return(jsonBlock, nil)
	mockRPCClient.EXPECT().Call(gomock.Any(), LastBlockRPC, nil).After(r7).Return(jsonHash, nil)
	log := common.SetupLogger(&common.LoggingOpts{
		Debug:   true,
		JSON:    false,
		Service: "test",
		Version: common.Version,
	})
	tracer := NewBlockTracer(mockRPCClient, mockStorage, log)
	tracer.Start(ctx, 500*time.Millisecond)
}

func TestTraceBlockJSONParse(t *testing.T) {
	var btr TraceBlockResponse

	raw := getJSONResult(t, "./testdata/trace_block.json")
	strResult := string(raw)
	reader := strings.NewReader(strResult)
	err := json.NewDecoder(reader).Decode(&btr)
	require.NoError(t, err)
	require.Len(t, btr, 989)
	for _, b := range btr {
		require.Equal(t, "0x4c2707c769754fe23764a765fc8d50a5fa4172a670e46671dd3e1c0c34036dfe", b.BlockHash)
	}
}

func getJSONResult(t *testing.T, filename string) json.RawMessage {
	t.Helper()
	f, err := os.Open(filename)
	require.NoError(t, err)
	defer f.Close()

	var rawjson map[string]json.RawMessage
	jsonb, err := io.ReadAll(f)
	require.NoError(t, err)

	err = json.Unmarshal(jsonb, &rawjson)
	require.NoError(t, err)

	result := rawjson["result"]
	require.NotEmpty(t, result)

	return result
}

func getJSON(t *testing.T, filename string) *rpcclient.RPCResponse {
	t.Helper()
	f, err := os.Open(filename)
	require.NoError(t, err)
	defer f.Close()

	jsonb, err := io.ReadAll(f)
	require.NoError(t, err)

	var resp *rpcclient.RPCResponse
	err = json.Unmarshal(jsonb, &resp)
	require.NoError(t, err)
	return resp
}
