package backend

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"

	abci "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	cosmosevmtypes "github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
)

func (s *TestSuite) TestGetTransactionByHash() {
	msgEthereumTx, _ := s.buildEthereumTx()
	txHash := msgEthereumTx.AsTransaction().Hash()

	txBz := s.signAndEncodeEthTx(msgEthereumTx)
	block := &types.Block{Header: types.Header{Height: 1, ChainID: "test"}, Data: types.Data{Txs: []types.Tx{txBz}}}
	responseDeliver := []*abci.ExecTxResult{
		{
			Code: 0,
			Events: []abci.Event{
				{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
					{Key: "ethereumTxHash", Value: txHash.Hex()},
					{Key: "txIndex", Value: "0"},
					{Key: "amount", Value: "1000"},
					{Key: "txGasUsed", Value: "21000"},
					{Key: "txHash", Value: ""},
					{Key: "recipient", Value: ""},
				}},
			},
		},
	}

	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx, common.Hash{}, 0, 0, big.NewInt(1), s.backend.EvmChainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Block error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, 1)
			},
			msgEthereumTx,
			rpcTransaction,
			false,
		},
		{
			"fail - Block Result error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlock(client, 1, txBz)
				s.Require().NoError(err)
				RegisterBlockResultsError(client, 1)
			},
			msgEthereumTx,
			nil,
			false,
		},
		{
			"pass - Base fee error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, txBz)
				s.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFeeError(QueryClient)
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
		{
			"pass - Transaction found and returned",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, txBz)
				s.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			db := dbm.NewMemDB()
			s.backend.Indexer = indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)
			err := s.backend.Indexer.IndexBlock(block, responseDeliver)
			s.Require().NoError(err)

			rpcTx, err := s.backend.GetTransactionByHash(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionsByHashPending() {
	msgEthereumTx, bz := s.buildEthereumTx()
	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx, common.Hash{}, 0, 0, big.NewInt(1), s.backend.EvmChainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Pending transactions returns error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxsError(client, nil)
			},
			msgEthereumTx,
			nil,
			true,
		},
		{
			"fail - Tx not found return nil",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxs(client, nil, nil)
			},
			msgEthereumTx,
			nil,
			true,
		},
		{
			"pass - Tx found and returned",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterUnconfirmedTxs(client, nil, types.Txs{bz})
			},
			msgEthereumTx,
			rpcTransaction,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := s.backend.GetTransactionByHashPending(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTxByEthHash() {
	msgEthereumTx, bz := s.buildEthereumTx()
	rpcTransaction, _ := rpctypes.NewRPCTransaction(msgEthereumTx, common.Hash{}, 0, 0, big.NewInt(1), s.backend.EvmChainID)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail - Indexer disabled can't find transaction",
			func() {
				s.backend.Indexer = nil
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				query := fmt.Sprintf("%s.%s='%s'", evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, common.HexToHash(msgEthereumTx.Hash).Hex())
				RegisterTxSearch(client, query, bz)
			},
			msgEthereumTx,
			rpcTransaction,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := s.backend.GetTxByEthHash(common.HexToHash(tc.tx.Hash))

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionByBlockHashAndIndex() {
	_, bz := s.buildEthereumTx()

	testCases := []struct {
		name         string
		registerMock func()
		blockHash    common.Hash
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"pass - block not found",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, common.Hash{}, bz)
			},
			common.Hash{},
			nil,
			true,
		},
		{
			"pass - Block results error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockByHash(client, common.Hash{}, bz)
				s.Require().NoError(err)
				RegisterBlockResultsError(client, 1)
			},
			common.Hash{},
			nil,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := s.backend.GetTransactionByBlockHashAndIndex(tc.blockHash, 1)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionByBlockAndIndex() {
	msgEthTx, bz := s.buildEthereumTx()

	defaultBlock := types.MakeBlock(1, []types.Tx{bz}, nil, nil)
	defaultExecTxResult := []*abci.ExecTxResult{
		{
			Code: 0,
			Events: []abci.Event{
				{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
					{Key: "ethereumTxHash", Value: common.HexToHash(msgEthTx.Hash).Hex()},
					{Key: "txIndex", Value: "0"},
					{Key: "amount", Value: "1000"},
					{Key: "txGasUsed", Value: "21000"},
					{Key: "txHash", Value: ""},
					{Key: "recipient", Value: ""},
				}},
			},
		},
	}

	txFromMsg, _ := rpctypes.NewTransactionFromMsg(
		msgEthTx,
		common.BytesToHash(defaultBlock.Hash().Bytes()),
		1,
		0,
		big.NewInt(1),
		s.backend.EvmChainID,
	)
	testCases := []struct {
		name         string
		registerMock func()
		block        *tmrpctypes.ResultBlock
		idx          hexutil.Uint
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"pass - block txs index out of bound",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				s.Require().NoError(err)
			},
			&tmrpctypes.ResultBlock{Block: types.MakeBlock(1, []types.Tx{bz}, nil, nil)},
			1,
			nil,
			true,
		},
		{
			"pass - Can't fetch base fee",
			func() {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFeeError(QueryClient)
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
		{
			"pass - Gets Tx by transaction index",
			func() {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				db := dbm.NewMemDB()
				s.backend.Indexer = indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)
				txBz := s.signAndEncodeEthTx(msgEthTx)
				block := &types.Block{Header: types.Header{Height: 1, ChainID: "test"}, Data: types.Data{Txs: []types.Tx{txBz}}}
				err := s.backend.Indexer.IndexBlock(block, defaultExecTxResult)
				s.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
		{
			"pass - returns the Ethereum format transaction by the Ethereum hash",
			func() {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			&tmrpctypes.ResultBlock{Block: defaultBlock},
			0,
			txFromMsg,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := s.backend.GetTransactionByBlockAndIndex(tc.block, tc.idx)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionByBlockNumberAndIndex() {
	msgEthTx, bz := s.buildEthereumTx()
	defaultBlock := types.MakeBlock(1, []types.Tx{bz}, nil, nil)
	txFromMsg, _ := rpctypes.NewTransactionFromMsg(
		msgEthTx,
		common.BytesToHash(defaultBlock.Hash().Bytes()),
		1,
		0,
		big.NewInt(1),
		s.backend.EvmChainID,
	)
	testCases := []struct {
		name         string
		registerMock func()
		blockNum     rpctypes.BlockNumber
		idx          hexutil.Uint
		expRPCTx     *rpctypes.RPCTransaction
		expPass      bool
	}{
		{
			"fail -  block not found return nil",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, 1)
			},
			0,
			0,
			nil,
			true,
		},
		{
			"pass - returns the transaction identified by block number and index",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				_, err := RegisterBlock(client, 1, bz)
				s.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				s.Require().NoError(err)
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			0,
			0,
			txFromMsg,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			rpcTx, err := s.backend.GetTransactionByBlockNumberAndIndex(tc.blockNum, tc.idx)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(rpcTx, tc.expRPCTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionByTxIndex() {
	_, bz := s.buildEthereumTx()

	testCases := []struct {
		name         string
		registerMock func()
		height       int64
		index        uint
		expTxResult  *cosmosevmtypes.TxResult
		expPass      bool
	}{
		{
			"fail - Ethereum tx with query not found",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				s.backend.Indexer = nil
				RegisterTxSearch(client, "tx.height=0 AND ethereum_tx.txIndex=0", bz)
			},
			0,
			0,
			&cosmosevmtypes.TxResult{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			txResults, err := s.backend.GetTxByTxIndex(tc.height, tc.index)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(txResults, tc.expTxResult)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestQueryTendermintTxIndexer() {
	testCases := []struct {
		name         string
		registerMock func()
		txGetter     func(*rpctypes.ParsedTxs) *rpctypes.ParsedTx
		query        string
		expTxResult  *cosmosevmtypes.TxResult
		expPass      bool
	}{
		{
			"fail - Ethereum tx with query not found",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterTxSearchEmpty(client, "")
			},
			func(_ *rpctypes.ParsedTxs) *rpctypes.ParsedTx {
				return &rpctypes.ParsedTx{}
			},
			"",
			&cosmosevmtypes.TxResult{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			txResults, err := s.backend.QueryTendermintTxIndexer(tc.query, tc.txGetter)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(txResults, tc.expTxResult)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetTransactionReceipt() {
	msgEthereumTx, _ := s.buildEthereumTx()
	msgEthereumTx2, _ := s.buildEthereumTx()
	txHash := msgEthereumTx.AsTransaction().Hash()
	txHash2 := msgEthereumTx2.AsTransaction().Hash()
	_ = txHash2

	txBz := s.signAndEncodeEthTx(msgEthereumTx)

	testCases := []struct {
		name         string
		registerMock func()
		tx           *evmtypes.MsgEthereumTx
		block        *types.Block
		blockResult  []*abci.ExecTxResult
		expPass      bool
		expErr       error
	}{
		// TODO test happy path
		{
			name:         "fail - tx not found",
			registerMock: func() {},
			block:        &types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			tx:           msgEthereumTx2,
			blockResult: []*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: txHash.Hex()},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			expPass: false,
			expErr:  fmt.Errorf("tx not found, hash: %s", txHash.Hex()),
		},
		{
			name: "fail - block not found",
			registerMock: func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				client.On("Block", mock.Anything, mock.Anything).Return(nil, errors.New("some error"))
			},
			block: &types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			tx:    msgEthereumTx,
			blockResult: []*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: txHash.Hex()},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			expPass: false,
			expErr:  fmt.Errorf("block not found at height 1: some error"),
		},
		{
			name: "fail - block result error",
			registerMock: func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				_, err := RegisterBlock(client, 1, txBz)
				s.Require().NoError(err)
				client.On("BlockResults", mock.Anything, mock.AnythingOfType("*int64")).
					Return(nil, errors.New("some error"))
			},
			tx:    msgEthereumTx,
			block: &types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			blockResult: []*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			expPass: false,
			expErr:  fmt.Errorf("block result not found at height 1: some error"),
		},
		{
			"happy path",
			func() {
				var header metadata.MD
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterParams(QueryClient, &header, 1)
				_, err := RegisterBlock(client, 1, txBz)
				s.Require().NoError(err)
				_, err = RegisterBlockResults(client, 1)
				s.Require().NoError(err)
			},
			msgEthereumTx,
			&types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			[]*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			true,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			tc.registerMock()

			db := dbm.NewMemDB()
			s.backend.Indexer = indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)
			err := s.backend.Indexer.IndexBlock(tc.block, tc.blockResult)
			s.Require().NoError(err)

			hash := common.HexToHash(tc.tx.Hash)
			res, err := s.backend.GetTransactionReceipt(hash)
			if tc.expPass {
				s.Require().Equal(res["transactionHash"], hash)
				s.Require().Equal(res["blockNumber"], hexutil.Uint64(tc.block.Height)) //nolint: gosec // G115
				requiredFields := []string{"status", "cumulativeGasUsed", "logsBloom", "logs", "gasUsed", "blockHash", "blockNumber", "transactionIndex", "effectiveGasPrice", "from", "to", "type"}
				for _, field := range requiredFields {
					s.Require().NotNil(res[field], "field was empty %s", field)
				}
				s.Require().Nil(res["contractAddress"]) // no contract creation
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expErr.Error())
			}
		})
	}
}

func (s *TestSuite) TestGetGasUsed() {
	origin := s.backend.Cfg.JSONRPC.FixRevertGasRefundHeight
	testCases := []struct {
		name                     string
		fixRevertGasRefundHeight int64
		txResult                 *cosmosevmtypes.TxResult
		price                    *big.Int
		gas                      uint64
		exp                      uint64
	}{
		{
			"success txResult",
			1,
			&cosmosevmtypes.TxResult{
				Height:  1,
				Failed:  false,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(0),
			0,
			53026,
		},
		{
			"fail txResult before cap",
			2,
			&cosmosevmtypes.TxResult{
				Height:  1,
				Failed:  true,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(200000),
			5000000000000,
			1000000000000000000,
		},
		{
			"fail txResult after cap",
			2,
			&cosmosevmtypes.TxResult{
				Height:  3,
				Failed:  true,
				GasUsed: 53026,
			},
			new(big.Int).SetUint64(200000),
			5000000000000,
			53026,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.backend.Cfg.JSONRPC.FixRevertGasRefundHeight = tc.fixRevertGasRefundHeight
			s.Require().Equal(tc.exp, s.backend.GetGasUsed(tc.txResult, tc.price, tc.gas))
			s.backend.Cfg.JSONRPC.FixRevertGasRefundHeight = origin
		})
	}
}
