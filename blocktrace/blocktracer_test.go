package blocktrace

import (
	"context"
	"encoding/json"
	"errors"
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

// TestBlockTrace() runs a number of tests for the main Tracer.Start loop
// It uses mocks to set and handle expected data.
func TestBlockTrace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())
	// Create a mock instance for the storage
	mockStorage := mocks.NewMockMEVTraceStorage(ctrl)
	// Sequence of expected calls on the storage mock:
	// First we return a fictitious number which will require to catch up.
	// The test loads a fixed json testdata file, which has the latest block set to 2391066
	// Therefore we will catcn up 3 blocks...
	s1 := mockStorage.EXPECT().LatestBlock().Return(uint64(22391064), nil)
	// ...so then we save 3 blocks...
	s2 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s1).Return(nil)
	s3 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s2).Return(nil)
	s4 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s3).Return(nil)
	// ...after which the loop will call for the latest block again.
	// THIS IS THE SIGNAL THAT EVERYTHING WENT WELL, so we call the cancel function of the ctx, which will stop the loop and finish the test
	mockStorage.EXPECT().LatestBlock().After(s4).Return(uint64(22391066), nil).Do(cancel)

	// create a mock instance for the RPC client
	mockRPCClient := mocks.NewMockRPCClient(ctrl)
	// Sequence: First the RPC client calls the last block RPC...
	jsonHash := getJSON(t, "./testdata/block_number.json")
	r1 := mockRPCClient.EXPECT().Call(gomock.Any(), LastBlockRPC, nil).Return(jsonHash, nil)
	// ...then it calls trace_block...
	jsonTrace := getJSON(t, "./testdata/trace_block.json")
	r2 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r1).Return(jsonTrace, nil)
	// ...and then calls the eth_getBlockByHash RPC
	jsonBlock := getJSON(t, "./testdata/block_hash.json")
	r3 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r2).Return(jsonBlock, nil)

	// After that, it calls 2 times more alternatively trace_block and block by hash, for each of the 3 blocks we fetch
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
	// Here is where the test actually starts!
	tracer.Start(ctx, 500*time.Millisecond)
}

func TestMissingBlock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(t.Context())
	// Create a mock instance for the storage
	mockStorage := mocks.NewMockMEVTraceStorage(ctrl)
	// Sequence of expected calls on the storage mock:
	// First we return a fictitious number which will require to catch up.
	// The test loads a fixed json testdata file, which has the latest block set to 2391066
	// This time we skip one, simulating a missing block.
	// Logic should not error and continue
	s1 := mockStorage.EXPECT().LatestBlock().Return(uint64(22391064), nil)
	// ...so then we save 2 blocks this time...(it's actually irrelevant, as we aren't really saving)
	s2 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s1).Return(nil)
	s4 := mockStorage.EXPECT().SaveMEVBLock(gomock.Any(), gomock.Any()).After(s2).Return(nil)
	// ...after which the loop will call for the latest block again.
	// THIS IS THE SIGNAL THAT EVERYTHING WENT WELL, so we call the cancel function of the ctx, which will stop the loop and finish the test
	mockStorage.EXPECT().LatestBlock().After(s4).Return(uint64(22391066), nil).Do(cancel)

	// create a mock instance for the RPC client
	mockRPCClient := mocks.NewMockRPCClient(ctrl)
	// Sequence: First the RPC client calls the last block RPC...
	jsonHash := getJSON(t, "./testdata/block_number.json")
	r1 := mockRPCClient.EXPECT().Call(gomock.Any(), LastBlockRPC, nil).Return(jsonHash, nil)
	// ...then it calls trace_block...
	jsonTrace := getJSON(t, "./testdata/trace_block.json")
	r2 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r1).Return(jsonTrace, nil)
	// ...and then calls the eth_getBlockByHash RPC
	jsonBlock := getJSON(t, "./testdata/block_hash.json")
	r3 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r2).Return(jsonBlock, nil)

	// After that, it calls 2 times TraceBlockRPC, but only 1 BlockByHashRPC, because one block can not be found
	r4 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r3).Return(nil, errors.New("mocking error"))
	r6 := mockRPCClient.EXPECT().Call(gomock.Any(), TraceBlockRPC, gomock.Any()).After(r4).Return(jsonTrace, nil)
	r7 := mockRPCClient.EXPECT().Call(gomock.Any(), BlockByHashRPC, gomock.Any()).After(r6).Return(jsonBlock, nil)
	mockRPCClient.EXPECT().Call(gomock.Any(), LastBlockRPC, nil).After(r7).Return(jsonHash, nil)
	log := common.SetupLogger(&common.LoggingOpts{
		Debug:   true,
		JSON:    false,
		Service: "test",
		Version: common.Version,
	})
	tracer := NewBlockTracer(mockRPCClient, mockStorage, log)
	// Here is where the test actually starts!
	tracer.Start(ctx, 500*time.Millisecond)
}

// TestTraceBlockJSONParse() just tests that we can parse the json response from trace_block
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

// getJSONResult() gets the "result" part as a json.RawMessage from a JSON response
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

// getJSON is used to get a RPCResponse after loading a test JSON file
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
