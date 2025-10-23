package vm

import (
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testKeyring "github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperTestSuite) TestContextSetConsensusParams() {
	// set new value of max gas in consensus params
	maxGas := int64(123456789)
	res, err := s.Network.App.GetConsensusParamsKeeper().Params(s.Network.GetContext(), &consensustypes.QueryParamsRequest{})
	s.Require().NoError(err)
	consParams := res.Params
	consParams.Block.MaxGas = maxGas
	_, err = s.Network.App.GetConsensusParamsKeeper().UpdateParams(s.Network.GetContext(), &consensustypes.MsgUpdateParams{
		Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		Block:     consParams.Block,
		Evidence:  consParams.Evidence,
		Validator: consParams.Validator,
		Abci:      consParams.Abci,
	})
	s.Require().NoError(err)

	queryContext := s.Network.GetQueryContext()
	proposerAddress := queryContext.BlockHeader().ProposerAddress
	cfg, err := s.Network.App.GetEVMKeeper().EVMConfig(queryContext, proposerAddress)
	s.Require().NoError(err)

	sender := s.Keyring.GetKey(0)
	recipient := s.Keyring.GetAddr(1)
	msg, err := s.Factory.GenerateGethCoreMsg(sender.Priv, types.EvmTxArgs{
		To:     &recipient,
		Amount: big.NewInt(100),
	})
	s.Require().NoError(err)

	// evm should query the max gas from consensus keeper, yielding the number set above.
	vm := s.Network.App.GetEVMKeeper().NewEVM(queryContext, *msg, cfg, nil, s.Network.GetStateDB())
	//nolint:gosec
	s.Require().Equal(vm.Context.GasLimit, uint64(maxGas))

	// if we explicitly set the consensus params in context, like when Cosmos builds a transaction context,
	// we should use that value, and not query the consensus params from the keeper.
	consParams.Block.MaxGas = 54321
	queryContext = queryContext.WithConsensusParams(*consParams)
	vm = s.Network.App.GetEVMKeeper().NewEVM(queryContext, *msg, cfg, nil, s.Network.GetStateDB())
	//nolint:gosec
	s.Require().Equal(vm.Context.GasLimit, uint64(consParams.Block.MaxGas))
}

func (s *KeeperTestSuite) TestGetHashFn() {
	s.SetupTest()
	header := s.Network.GetContext().BlockHeader()
	h, _ := cmttypes.HeaderFromProto(&header)
	hash := h.Hash()

	testCases := []struct {
		msg      string
		height   uint64
		malleate func() sdk.Context
		expHash  common.Hash
	}{
		{
			"case 1.1: context hash cached",
			uint64(s.Network.GetContext().BlockHeight()), //nolint:gosec // G115
			func() sdk.Context {
				return s.Network.GetContext().WithHeaderHash(
					tmhash.Sum([]byte("header")),
				)
			},
			common.BytesToHash(tmhash.Sum([]byte("header"))),
		},
		{
			"case 1.2: failed to cast CometBFT header",
			uint64(s.Network.GetContext().BlockHeight()), //nolint:gosec // G115
			func() sdk.Context {
				header := tmproto.Header{}
				header.Height = s.Network.GetContext().BlockHeight()
				return s.Network.GetContext().WithBlockHeader(header)
			},
			common.Hash{},
		},
		{
			"case 1.3: hash calculated from CometBFT header",
			uint64(s.Network.GetContext().BlockHeight()), //nolint:gosec // G115
			func() sdk.Context {
				return s.Network.GetContext().WithBlockHeader(header)
			},
			common.BytesToHash(hash),
		},
		{
			"case 2.1: height lower than current one, hist info not found",
			1,
			func() sdk.Context {
				return s.Network.GetContext().WithBlockHeight(10)
			},
			common.Hash{},
		},
		{
			"case 2.2: height lower than current one, invalid hist info header",
			1,
			func() sdk.Context {
				s.Require().NoError(s.Network.App.GetStakingKeeper().SetHistoricalInfo(s.Network.GetContext(), 1, &stakingtypes.HistoricalInfo{}))
				return s.Network.GetContext().WithBlockHeight(10)
			},
			common.Hash{},
		},
		{
			"case 2.3: height lower than current one, calculated from hist info header",
			1,
			func() sdk.Context {
				histInfo := &stakingtypes.HistoricalInfo{
					Header: header,
				}
				s.Require().NoError(s.Network.App.GetStakingKeeper().SetHistoricalInfo(s.Network.GetContext(), 1, histInfo))
				return s.Network.GetContext().WithBlockHeight(10)
			},
			common.BytesToHash(hash),
		},
		{
			"case 3: height greater than current one",
			200,
			func() sdk.Context { return s.Network.GetContext() },
			common.Hash{},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			ctx := tc.malleate()

			// Function being tested
			hash := s.Network.App.GetEVMKeeper().GetHashFn(ctx)(tc.height)
			s.Require().Equal(tc.expHash, hash)

			err := s.Network.NextBlock()
			s.Require().NoError(err)
		})
	}
}

func (s *KeeperTestSuite) TestGetCoinbaseAddress() {
	s.SetupTest()
	validators := s.Network.GetValidators()
	proposerAddressHex := utils.ValidatorConsAddressToHex(
		validators[0].OperatorAddress,
	)

	testCases := []struct {
		msg      string
		malleate func() sdk.Context
		expPass  bool
	}{
		{
			"validator not found",
			func() sdk.Context {
				header := s.Network.GetContext().BlockHeader()
				header.ProposerAddress = []byte{}
				return s.Network.GetContext().WithBlockHeader(header)
			},
			false,
		},
		{
			"success",
			func() sdk.Context {
				return s.Network.GetContext()
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			ctx := tc.malleate()
			proposerAddress := ctx.BlockHeader().ProposerAddress

			// Function being tested
			coinbase, err := s.Network.App.GetEVMKeeper().GetCoinbaseAddress(
				ctx,
				sdk.ConsAddress(proposerAddress),
			)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(proposerAddressHex, coinbase)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetEthIntrinsicGas() {
	s.SetupTest()
	testCases := []struct {
		name               string
		data               []byte
		accessList         gethtypes.AccessList
		height             int64
		isContractCreation bool
		noError            bool
		expGas             uint64
	}{
		{
			"no data, no accesslist, not contract creation, not homestead, not istanbul, not shanghai",
			nil,
			nil,
			1,
			false,
			true,
			params.TxGas,
		},
		{
			"with one zero data, no accesslist, not contract creation, not homestead, not istanbul, not shanghai",
			[]byte{0},
			nil,
			1,
			false,
			true,
			params.TxGas + params.TxDataZeroGas*1,
		},
		{
			"with one non zero data, no accesslist, not contract creation, not homestead, not istanbul, not shanghai",
			[]byte{1},
			nil,
			1,
			true,
			true,
			params.TxGas + params.TxDataNonZeroGasFrontier*1,
		},
		{
			"no data, one accesslist, not contract creation, not homestead, not istanbul, not shanghai",
			nil,
			[]gethtypes.AccessTuple{
				{},
			},
			1,
			false,
			true,
			params.TxGas + params.TxAccessListAddressGas,
		},
		{
			"no data, one accesslist with one storageKey, not contract creation, not homestead, not istanbul, not shanghai",
			nil,
			[]gethtypes.AccessTuple{
				{StorageKeys: make([]common.Hash, 1)},
			},
			1,
			false,
			true,
			params.TxGas + params.TxAccessListAddressGas + params.TxAccessListStorageKeyGas*1,
		},
		{
			"no data, no accesslist, is contract creation, is homestead, not istanbul, not shanghai",
			nil,
			nil,
			2,
			true,
			true,
			params.TxGasContractCreation,
		},
		{
			"with one zero data, no accesslist, not contract creation, is homestead, is istanbul, not shanghai",
			[]byte{1},
			nil,
			3,
			false,
			true,
			params.TxGas + params.TxDataNonZeroGasEIP2028*1,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			ethCfg := types.GetEthChainConfig()
			ethCfg.HomesteadBlock = big.NewInt(2)
			ethCfg.IstanbulBlock = big.NewInt(3)
			signer := gethtypes.LatestSignerForChainID(ethCfg.ChainID)

			// in the future, fork not enabled
			shanghaiTime := uint64(s.Network.GetContext().BlockTime().Unix()) + 10000 //#nosec G115 -- int overflow is not a concern here
			ethCfg.ShanghaiTime = &shanghaiTime

			ctx := s.Network.GetContext().WithBlockHeight(tc.height)

			addr := s.Keyring.GetAddr(0)
			krSigner := utiltx.NewSigner(s.Keyring.GetPrivKey(0))
			nonce := s.Network.App.GetEVMKeeper().GetNonce(ctx, addr)
			m, err := newNativeMessage(
				nonce,
				addr,
				krSigner,
				signer,
				gethtypes.AccessListTxType,
				tc.data,
				tc.accessList,
				nil,
			)
			s.Require().NoError(err)

			// Function being tested
			gas, err := s.Network.App.GetEVMKeeper().GetEthIntrinsicGas(
				ctx,
				*m,
				ethCfg,
				tc.isContractCreation,
			)

			if tc.noError {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}

			s.Require().Equal(tc.expGas, gas)
		})
	}
}

func (s *KeeperTestSuite) TestGasToRefund() {
	s.SetupTest()
	testCases := []struct {
		name           string
		gasconsumed    uint64
		refundQuotient uint64
		expGasRefund   uint64
		expPanic       bool
	}{
		{
			"gas refund 5",
			5,
			1,
			5,
			false,
		},
		{
			"gas refund 10",
			10,
			1,
			10,
			false,
		},
		{
			"gas refund availableRefund",
			11,
			1,
			10,
			false,
		},
		{
			"gas refund quotient 0",
			11,
			0,
			0,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			vmdb := s.Network.GetStateDB()
			vmdb.AddRefund(10)

			if tc.expPanic {
				panicF := func() {
					keeper.GasToRefund(vmdb.GetRefund(), tc.gasconsumed, tc.refundQuotient)
				}
				s.Require().Panics(panicF)
			} else {
				gr := keeper.GasToRefund(vmdb.GetRefund(), tc.gasconsumed, tc.refundQuotient)
				s.Require().Equal(tc.expGasRefund, gr)
			}
		})
	}
}

func (s *KeeperTestSuite) TestRefundGas() {
	// FeeCollector account is pre-funded with enough tokens
	// for refund to work
	// NOTE: everything should happen within the same block for
	// feecollector account to remain funded
	baseDenom := types.GetEVMCoinDenom()

	coins := sdk.NewCoins(sdk.NewCoin(
		baseDenom,
		sdkmath.NewInt(6e18),
	))
	balances := []banktypes.Balance{
		{
			Address: authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			Coins:   coins,
		},
	}
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = balances
	customGenesis := network.CustomGenesisState{}
	customGenesis[banktypes.ModuleName] = bankGenesis

	Keyring := testKeyring.New(2)
	unitNetwork := network.NewUnitTestNetwork(
		s.Create,
		network.WithPreFundedAccounts(Keyring.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	txFactory := factory.New(unitNetwork, grpcHandler)

	sender := Keyring.GetKey(0)
	recipient := Keyring.GetAddr(1)

	testCases := []struct {
		name           string
		leftoverGas    uint64
		refundQuotient uint64
		noError        bool
		expGasRefund   uint64
		gasPrice       *big.Int
	}{
		{
			name:           "leftoverGas more than tx gas limit",
			leftoverGas:    params.TxGas + 1,
			refundQuotient: params.RefundQuotient,
			noError:        false,
			expGasRefund:   params.TxGas + 1,
		},
		{
			name:           "leftoverGas equal to tx gas limit, insufficient fee collector account",
			leftoverGas:    params.TxGas,
			refundQuotient: params.RefundQuotient,
			noError:        true,
			expGasRefund:   0,
		},
		{
			name:           "leftoverGas less than to tx gas limit",
			leftoverGas:    params.TxGas - 1,
			refundQuotient: params.RefundQuotient,
			noError:        true,
			expGasRefund:   0,
		},
		{
			name:           "no leftoverGas, refund half used gas ",
			leftoverGas:    0,
			refundQuotient: params.RefundQuotient,
			noError:        true,
			expGasRefund:   params.TxGas / params.RefundQuotient,
		},
		{
			name:           "invalid GasPrice in message",
			leftoverGas:    0,
			refundQuotient: params.RefundQuotient,
			noError:        false,
			expGasRefund:   params.TxGas / params.RefundQuotient,
			gasPrice:       big.NewInt(-100),
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			coreMsg, err := txFactory.GenerateGethCoreMsg(
				sender.Priv,
				types.EvmTxArgs{
					To:       &recipient,
					Amount:   big.NewInt(100),
					GasPrice: tc.gasPrice,
				},
			)
			s.Require().NoError(err)
			transactionGas := coreMsg.GasLimit

			vmdb := unitNetwork.GetStateDB()
			vmdb.AddRefund(params.TxGas)

			if tc.leftoverGas > transactionGas {
				return
			}

			gasUsed := transactionGas - tc.leftoverGas
			refund := keeper.GasToRefund(vmdb.GetRefund(), gasUsed, tc.refundQuotient)
			s.Require().Equal(tc.expGasRefund, refund)

			err = unitNetwork.App.GetEVMKeeper().RefundGas(
				unitNetwork.GetContext(),
				*coreMsg,
				refund,
				unitNetwork.GetBaseDenom(),
			)

			if tc.noError {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestResetGasMeterAndConsumeGas() {
	s.SetupTest()
	testCases := []struct {
		name        string
		gasConsumed uint64
		gasUsed     uint64
		expPanic    bool
	}{
		{
			"gas consumed 5, used 5",
			5,
			5,
			false,
		},
		{
			"gas consumed 5, used 10",
			5,
			10,
			false,
		},
		{
			"gas consumed 10, used 10",
			10,
			10,
			false,
		},
		{
			"gas consumed 11, used 10, NegativeGasConsumed panic",
			11,
			10,
			true,
		},
		{
			"gas consumed 1, used 10, overflow panic",
			1,
			math.MaxUint64,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			panicF := func() {
				gm := storetypes.NewGasMeter(10)
				gm.ConsumeGas(tc.gasConsumed, "")
				ctx := s.Network.GetContext().WithGasMeter(gm)
				s.Network.App.GetEVMKeeper().ResetGasMeterAndConsumeGas(ctx, tc.gasUsed)
			}

			if tc.expPanic {
				s.Require().Panics(panicF)
			} else {
				s.Require().NotPanics(panicF)
			}
		})
	}
}

func (s *KeeperTestSuite) TestEVMConfig() {
	s.SetupTest()

	defaultChainEVMParams := config.NewEVMGenesisState().Params

	proposerAddress := s.Network.GetContext().BlockHeader().ProposerAddress
	cfg, err := s.Network.App.GetEVMKeeper().EVMConfig(
		s.Network.GetContext(),
		proposerAddress,
	)
	s.Require().NoError(err)
	s.Require().Equal(defaultChainEVMParams, cfg.Params)
	// london hardfork is enabled by default
	s.Require().Equal(big.NewInt(0), cfg.BaseFee)

	validators := s.Network.GetValidators()
	proposerHextAddress := utils.ValidatorConsAddressToHex(validators[0].OperatorAddress)
	s.Require().Equal(proposerHextAddress, cfg.CoinBase)
}

func (s *KeeperTestSuite) TestApplyTransaction() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	// FeeCollector account is pre-funded with enough tokens
	// for refund to work
	// NOTE: everything should happen within the same block for
	// feecollector account to remain funded
	s.SetupTest()
	// set bounded cosmos block gas limit
	ctx := s.Network.GetContext().WithBlockGasMeter(storetypes.NewGasMeter(1e6))
	err := s.Network.App.GetBankKeeper().MintCoins(ctx, "mint", sdk.NewCoins(sdk.NewCoin("aatom", sdkmath.NewInt(3e18))))
	s.Require().NoError(err)
	err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, "mint", "fee_collector", sdk.NewCoins(sdk.NewCoin("aatom", sdkmath.NewInt(3e18))))
	s.Require().NoError(err)
	testCases := []struct {
		name       string
		gasLimit   uint64
		requireErr bool
		errorMsg   string
	}{
		{
			"pass - set evm limit above cosmos block gas limit and refund",
			6e6,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), types.EvmTxArgs{
				GasLimit: tc.gasLimit,
			})
			s.Require().NoError(err)
			initialBalance := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(0), "aatom")

			ethMsg := tx.GetMsgs()[0].(*types.MsgEthereumTx)
			res, err := s.Network.App.GetEVMKeeper().ApplyTransaction(ctx, ethMsg.AsTransaction())
			s.Require().NoError(err)
			s.Require().Equal(res.GasUsed, uint64(3e6))
			// Half of the gas should be refunded based on the protocol refund cap.
			// Note that the balance should only increment by the refunded amount
			// because ApplyTransaction does not consume and take the gas from the user.
			balanceAfterRefund := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(0), "aatom")
			expectedRefund := new(big.Int).Mul(new(big.Int).SetUint64(6e6/2), s.Network.App.GetEVMKeeper().GetBaseFee(ctx))
			s.Require().Equal(balanceAfterRefund.Sub(initialBalance).Amount, sdkmath.NewIntFromBigInt(expectedRefund))
		})
	}
}

func (s *KeeperTestSuite) TestApplyMessage() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	// Generate a transfer tx message
	sender := s.Keyring.GetKey(0)
	recipient := s.Keyring.GetAddr(1)
	transferArgs := types.EvmTxArgs{
		To:     &recipient,
		Amount: big.NewInt(100),
	}
	coreMsg, err := s.Factory.GenerateGethCoreMsg(
		sender.Priv,
		transferArgs,
	)
	s.Require().NoError(err)

	tracer := s.Network.App.GetEVMKeeper().Tracer(
		s.Network.GetContext(),
		*coreMsg,
		types.GetEthChainConfig(),
	)
	res, err := s.Network.App.GetEVMKeeper().ApplyMessage(s.Network.GetContext(), *coreMsg, tracer, true, false)
	s.Require().NoError(err)
	s.Require().False(res.Failed())

	// Compare gas to a transfer tx gas
	expectedGasUsed := params.TxGas
	s.Require().Equal(expectedGasUsed, res.GasUsed)
}

func (s *KeeperTestSuite) TestApplyMessageWithConfig() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()
	testCases := []struct {
		name               string
		getMessage         func() core.Message
		getEVMParams       func() types.Params
		getFeeMarketParams func() feemarkettypes.Params
		expErr             bool
		expVMErr           bool
		expectedGasUsed    uint64
	}{
		{
			"success - messsage applied ok with default params",
			func() core.Message {
				sender := s.Keyring.GetKey(0)
				recipient := s.Keyring.GetAddr(1)
				msg, err := s.Factory.GenerateGethCoreMsg(sender.Priv, types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(100),
				})
				s.Require().NoError(err)
				return *msg
			},
			types.DefaultParams,
			feemarkettypes.DefaultParams,
			false,
			false,
			params.TxGas,
		},
		{
			"call contract tx with config param EnableCall = false",
			func() core.Message {
				sender := s.Keyring.GetKey(0)
				recipient := s.Keyring.GetAddr(1)
				msg, err := s.Factory.GenerateGethCoreMsg(sender.Priv, types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(100),
					Input:  []byte("contract_data"),
				})
				s.Require().NoError(err)
				return *msg
			},
			func() types.Params {
				defaultParams := types.DefaultParams()
				defaultParams.AccessControl = types.AccessControl{
					Call: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				return defaultParams
			},
			feemarkettypes.DefaultParams,
			false,
			true,
			0,
		},
		{
			"create contract tx with config param EnableCreate = false",
			func() core.Message {
				sender := s.Keyring.GetKey(0)
				msg, err := s.Factory.GenerateGethCoreMsg(sender.Priv, types.EvmTxArgs{
					Amount: big.NewInt(100),
					Input:  []byte("contract_data"),
				})
				s.Require().NoError(err)
				return *msg
			},
			func() types.Params {
				defaultParams := types.DefaultParams()
				defaultParams.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				return defaultParams
			},
			feemarkettypes.DefaultParams,
			false,
			true,
			0,
		},
		{
			"fail - fix panic when minimumGasUsed is not uint64",
			func() core.Message {
				sender := s.Keyring.GetKey(0)
				recipient := s.Keyring.GetAddr(1)
				msg, err := s.Factory.GenerateGethCoreMsg(sender.Priv, types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(100),
				})
				s.Require().NoError(err)
				return *msg
			},
			types.DefaultParams,
			func() feemarkettypes.Params {
				paramsRes, err := s.Handler.GetFeeMarketParams()
				s.Require().NoError(err)
				params := paramsRes.GetParams()
				params.MinGasMultiplier = sdkmath.LegacyNewDec(math.MaxInt64).MulInt64(100)
				return params
			},
			true,
			false,
			0,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			msg := tc.getMessage()
			evmParams := tc.getEVMParams()
			err := s.Network.App.GetEVMKeeper().SetParams(
				s.Network.GetContext(),
				evmParams,
			)
			s.Require().NoError(err)
			feeMarketparams := tc.getFeeMarketParams()
			err = s.Network.App.GetFeeMarketKeeper().SetParams(
				s.Network.GetContext(),
				feeMarketparams,
			)
			s.Require().NoError(err)

			txConfig := s.Network.App.GetEVMKeeper().TxConfig(
				s.Network.GetContext(),
				common.Hash{},
			)
			proposerAddress := s.Network.GetContext().BlockHeader().ProposerAddress
			config, err := s.Network.App.GetEVMKeeper().EVMConfig(
				s.Network.GetContext(),
				proposerAddress,
			)
			s.Require().NoError(err)

			// Function being tested
			res, err := s.Network.App.GetEVMKeeper().ApplyMessageWithConfig(s.Network.GetContext(), msg, nil, true, config, txConfig, false)

			if tc.expErr {
				s.Require().Error(err)
			} else if !tc.expVMErr {
				s.Require().NoError(err)
				s.Require().False(res.Failed())
				s.Require().Equal(tc.expectedGasUsed, res.GasUsed)
			}

			err = s.Network.NextBlock()
			if tc.expVMErr {
				s.Require().NotEmpty(res.VmError)
				return
			}

			if tc.expVMErr {
				s.Require().NotEmpty(res.VmError)
				return
			}

			s.Require().NoError(err)
		})
	}
}

func (s *KeeperTestSuite) TestGetProposerAddress() {
	s.SetupTest()
	address := sdk.ConsAddress(s.Keyring.GetAddr(0).Bytes())
	proposerAddress := sdk.ConsAddress(s.Network.GetContext().BlockHeader().ProposerAddress)
	testCases := []struct {
		msg    string
		addr   sdk.ConsAddress
		expAdr sdk.ConsAddress
	}{
		{
			"proposer address provided",
			address,
			address,
		},
		{
			"nil proposer address provided",
			nil,
			proposerAddress,
		},
		{
			"typed nil proposer address provided",
			sdk.ConsAddress{},
			proposerAddress,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			s.Require().Equal(
				tc.expAdr,
				keeper.GetProposerAddress(s.Network.GetContext(), tc.addr),
			)
		})
	}
}

func (s *KeeperTestSuite) TestApplyMessageWithNegativeAmount() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	// Generate a transfer tx message
	sender := s.Keyring.GetKey(0)
	recipient := s.Keyring.GetAddr(1)
	amt, _ := big.NewInt(0).SetString("-115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)
	transferArgs := types.EvmTxArgs{
		To:     &recipient,
		Amount: amt,
	}
	coreMsg, err := s.Factory.GenerateGethCoreMsg(
		sender.Priv,
		transferArgs,
	)
	s.Require().NoError(err)

	tracer := s.Network.App.GetEVMKeeper().Tracer(
		s.Network.GetContext(),
		*coreMsg,
		types.GetEthChainConfig(),
	)

	ctx := s.Network.GetContext()
	balance0Before := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(0), "aatom")
	balance1Before := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(1), "aatom")
	res, err := s.Network.App.GetEVMKeeper().ApplyMessage(
		s.Network.GetContext(),
		*coreMsg,
		tracer,
		true,
		false,
	)
	s.Require().Nil(res)
	s.Require().Error(err)

	balance0After := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(0), "aatom")
	balance1After := s.Network.App.GetBankKeeper().GetBalance(ctx, s.Keyring.GetAccAddr(1), "aatom")

	s.Require().Equal(balance0Before, balance0After)
	s.Require().Equal(balance1Before, balance1After)
}
