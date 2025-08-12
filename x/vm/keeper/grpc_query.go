package keeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	ethparams "github.com/ethereum/go-ethereum/params"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	cosmosevmtypes "github.com/cosmos/evm/types"
	evmante "github.com/cosmos/evm/x/vm/ante"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

const (
	defaultTraceTimeout = 5 * time.Second
	// maxTracePredecessors is the maximum amount of transaction predecessors to be included in a trace Tx request.
	// This limit is chosen as a sensible default to prevent unbounded predecessor iteration.
	maxTracePredecessors = 10_000

	maxPredecessorGas = uint64(50_000_000)
)

// Account implements the Query/Account gRPC method. The method returns the
// *spendable* balance of the account in 18 decimals representation.
func (k Keeper) Account(c context.Context, req *types.QueryAccountRequest) (*types.QueryAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := cosmosevmtypes.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	addr := common.HexToAddress(req.Address)

	ctx := sdk.UnwrapSDKContext(c)
	acct := k.GetAccountOrEmpty(ctx, addr)

	return &types.QueryAccountResponse{
		Balance:  acct.Balance.String(),
		CodeHash: common.BytesToHash(acct.CodeHash).Hex(),
		Nonce:    acct.Nonce,
	}, nil
}

func (k Keeper) CosmosAccount(c context.Context, req *types.QueryCosmosAccountRequest) (*types.QueryCosmosAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := cosmosevmtypes.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	ethAddr := common.HexToAddress(req.Address)
	cosmosAddr := sdk.AccAddress(ethAddr.Bytes())

	account := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	res := types.QueryCosmosAccountResponse{
		CosmosAddress: cosmosAddr.String(),
	}

	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

// ValidatorAccount implements the Query/Balance gRPC method
func (k Keeper) ValidatorAccount(c context.Context, req *types.QueryValidatorAccountRequest) (*types.QueryValidatorAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	consAddr, err := sdk.ConsAddressFromBech32(req.ConsAddress)
	if err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	validator, err := k.stakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
	if err != nil {
		return nil, fmt.Errorf("error while getting validator %s. %w", consAddr.String(), err)
	}

	valAddr := validator.GetOperator()
	addrBz, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(valAddr)
	if err != nil {
		return nil, fmt.Errorf("error while getting validator %s. %w", consAddr.String(), err)
	}

	res := types.QueryValidatorAccountResponse{
		AccountAddress: sdk.AccAddress(addrBz).String(),
	}

	account := k.accountKeeper.GetAccount(ctx, addrBz)
	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

// Balance implements the Query/Balance gRPC method. The method returns the 18
// decimal representation of the account's *spendable* balance.
func (k Keeper) Balance(c context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := cosmosevmtypes.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	balanceInt := k.SpendableCoin(ctx, common.HexToAddress(req.Address))

	return &types.QueryBalanceResponse{
		Balance: balanceInt.String(),
	}, nil
}

// Storage implements the Query/Storage gRPC method
func (k Keeper) Storage(c context.Context, req *types.QueryStorageRequest) (*types.QueryStorageResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := cosmosevmtypes.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	key := common.HexToHash(req.Key)

	state := k.GetState(ctx, address, key)
	stateHex := state.Hex()

	return &types.QueryStorageResponse{
		Value: stateHex,
	}, nil
}

// Code implements the Query/Code gRPC method
func (k Keeper) Code(c context.Context, req *types.QueryCodeRequest) (*types.QueryCodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := cosmosevmtypes.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	acct := k.GetAccountWithoutBalance(ctx, address)

	var code []byte
	if acct != nil && acct.IsContract() {
		code = k.GetCode(ctx, common.BytesToHash(acct.CodeHash))
	}

	return &types.QueryCodeResponse{
		Code: code,
	}, nil
}

// Params implements the Query/Params gRPC method
func (k Keeper) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// EthCall implements eth_call rpc api.
func (k Keeper) EthCall(c context.Context, req *types.EthCallRequest) (*types.MsgEthereumTxResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	var args types.TransactionArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ApplyMessageWithConfig expect correct nonce set in msg
	nonce := k.GetNonce(ctx, args.GetFrom())
	args.Nonce = (*hexutil.Uint64)(&nonce)

	msg, err := args.ToMessage(req.GasCap, cfg.BaseFee, false, false)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	txConfig := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash()))

	// pass false to not commit StateDB
	res, err := k.ApplyMessageWithConfig(ctx, msg, nil, false, cfg, txConfig, false)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

// EstimateGas implements eth_estimateGas rpc api.
func (k Keeper) EstimateGas(c context.Context, req *types.EthCallRequest) (*types.EstimateGasResponse, error) {
	return k.EstimateGasInternal(c, req, types.RPC)
}

// EstimateGasInternal returns the gas estimation for the corresponding request.
// This function is called from the RPC client (eth_estimateGas) and internally.
// When called from the RPC client, we need to reset the gas meter before
// simulating the transaction to have
// an accurate gas estimation for EVM extensions transactions.
func (k Keeper) EstimateGasInternal(c context.Context, req *types.EthCallRequest, fromType types.CallType) (*types.EstimateGasResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	ctx := sdk.UnwrapSDKContext(c)

	if req.GasCap < ethparams.TxGas {
		return nil, status.Errorf(codes.InvalidArgument, "gas cap cannot be lower than %d", ethparams.TxGas)
	}

	var args types.TransactionArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo     = ethparams.TxGas - 1
		hi     uint64
		gasCap uint64
	)

	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= ethparams.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Query block gas limit
		params := ctx.ConsensusParams()
		if params.Block != nil && params.Block.MaxGas > 0 {
			hi = uint64(params.Block.MaxGas) //nolint:gosec // G115 // won't exceed uint64
		} else {
			hi = req.GasCap
		}
	}

	// Recap the highest gas allowance with specified gascap.
	if req.GasCap != 0 && hi > req.GasCap {
		hi = req.GasCap
	}

	gasCap = hi
	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load evm config")
	}

	// ApplyMessageWithConfig expect correct nonce set in msg
	nonce := k.GetNonce(ctx, args.GetFrom())
	args.Nonce = (*hexutil.Uint64)(&nonce)

	txConfig := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash()))

	// convert the tx args to an ethereum message
	msg, err := args.ToMessage(req.GasCap, cfg.BaseFee, true, true)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Recap the highest gas limit with account's available balance.
	if msg.GasFeeCap.BitLen() != 0 {
		baseDenom := types.GetEVMCoinDenom()

		balance := k.bankWrapper.SpendableCoin(ctx, sdk.AccAddress(args.From.Bytes()), baseDenom)
		available := balance.Amount
		transfer := "0"
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available.BigInt()) >= 0 {
				return nil, core.ErrInsufficientFundsForTransfer
			}
			available = available.Sub(sdkmath.NewIntFromBigInt(args.Value.ToInt()))
			transfer = args.Value.String()
		}
		allowance := available.Quo(sdkmath.NewIntFromBigInt(msg.GasFeeCap))

		// If the allowance is larger than maximum uint64, skip checking
		if allowance.IsUint64() && hi > allowance.Uint64() {
			k.Logger(ctx).Debug("Gas estimation capped by limited funds", "original", hi, "balance", balance,
				"sent", transfer, "maxFeePerGas", msg.GasFeeCap.String(), "fundable", allowance)

			hi = allowance.Uint64()
			if hi < ethparams.TxGas {
				return nil, core.ErrInsufficientFunds
			}
		}
	}

	// NOTE: the errors from the executable below should be consistent with go-ethereum,
	// so we don't wrap them with the gRPC status code

	// create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (vmError bool, rsp *types.MsgEthereumTxResponse, err error) {
		// update the message with the new gas value
		msg.GasLimit = gas

		tmpCtx := ctx
		if fromType == types.RPC {
			tmpCtx, _ = ctx.CacheContext()

			acct := k.GetAccount(tmpCtx, msg.From)

			from := msg.From
			if acct == nil {
				acc := k.accountKeeper.NewAccountWithAddress(tmpCtx, from[:])
				k.accountKeeper.SetAccount(tmpCtx, acc)
				acct = statedb.NewEmptyAccount()
			}
			// When submitting a transaction, the `EthIncrementSenderSequence` ante handler increases the account nonce
			acct.Nonce = nonce + 1
			err = k.SetAccount(tmpCtx, from, *acct)
			if err != nil {
				return true, nil, err
			}
			// resetting the gasMeter after increasing the sequence to have an accurate gas estimation on EVM extensions transactions
			tmpCtx = buildTraceCtx(tmpCtx, msg.GasLimit)
		}
		// pass false to not commit StateDB
		rsp, err = k.ApplyMessageWithConfig(tmpCtx, msg, nil, false, cfg, txConfig, false)
		if err != nil {
			if errors.Is(err, core.ErrIntrinsicGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return len(rsp.VmError) > 0, rsp, nil
	}

	// Execute the binary search and hone in on an executable gas limit
	hi, err = types.BinSearch(lo, hi, executable)
	if err != nil {
		return nil, err
	}

	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == gasCap {
		failed, result, err := executable(hi)
		if err != nil {
			return nil, err
		}

		if failed {
			if result != nil && result.VmError != vm.ErrOutOfGas.Error() {
				if result.VmError == vm.ErrExecutionReverted.Error() {
					return &types.EstimateGasResponse{
						Ret:     result.Ret,
						VmError: result.VmError,
					}, nil
				}
				return nil, errors.New(result.VmError)
			}
			// Otherwise, the specified gas cap is too low
			return nil, fmt.Errorf("gas required exceeds allowance (%d)", gasCap)
		}
	}
	return &types.EstimateGasResponse{Gas: hi}, nil
}

// TraceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer-dependent.
func (k Keeper) TraceTx(c context.Context, req *types.QueryTraceTxRequest) (*types.QueryTraceTxResponse, error) {
	if req == nil || req.Msg == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	if len(req.Predecessors) > maxTracePredecessors {
		return nil, status.Errorf(codes.InvalidArgument, "too many predecessors, got %d: limit %d", len(req.Predecessors), maxTracePredecessors)
	}

	// get the context of block beginning
	requestedHeight := req.BlockNumber
	if requestedHeight < 1 {
		// 0 is a special value in `ContextWithHeight`
		requestedHeight = 1
	}

	ctx := sdk.UnwrapSDKContext(c)
	// the caller sets the `ctx.BlockHeight()` to be `requestedHeight - 1`, so we can get the context of block beginning
	if requestedHeight > ctx.BlockHeight()+1 {
		return nil, status.Errorf(codes.FailedPrecondition, "requested height [%d] must be less than or equal to current height [%d]", requestedHeight, ctx.BlockHeight())
	}
	// TODO: ideally, this query should validate that the block hash, predecessor transactions, and main trace tx actually existed in the block requested.
	// These fields should not be defined by a user.
	// For now, since the backend uses this query to run its traces, we cannot do much here. This needs to be refactored
	// so that backend isn't using this method over gRPC, but directly.
	// see: https://linear.app/cosmoslabs/issue/EVM-149/some-xvm-queries-are-meant-to-be-internal
	ctx = ctx.WithBlockHeight(requestedHeight)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	// to get the base fee we only need the block max gas in the consensus params
	ctx = ctx.WithConsensusParams(tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{MaxGas: req.BlockMaxGas},
	})

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load evm config: %s", err.Error())
	}

	// compute and use base fee of the height that is being traced
	baseFee := k.feeMarketWrapper.CalculateBaseFee(ctx)
	if baseFee != nil {
		cfg.BaseFee = baseFee
	}

	signer := ethtypes.MakeSigner(types.GetEthChainConfig(), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	txConfig := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash()))

	// gas used at this point corresponds to GetProposerAddress & CalculateBaseFee
	// need to reset gas meter per transaction to be consistent with tx execution
	// and avoid stacking the gas used of every predecessor in the same gas meter

	ctx = evmante.BuildEvmExecutionCtx(ctx).
		WithGasMeter(storetypes.NewGasMeter(maxPredecessorGas))
	for i, tx := range req.Predecessors {
		ethTx := tx.AsTransaction()
		var msg *core.Message
		// if tx is not unsigned, from field should be derived from signer, which can be done using AsMessage function
		if !isUnsigned(ethTx) {
			msg, err = core.TransactionToMessage(ethTx, signer, cfg.BaseFee)
			if err != nil {
				continue
			}
		} else {
			msg = unsignedTxAsMessage(common.BytesToAddress(req.Msg.From), ethTx, cfg.BaseFee)
		}
		msg.GasLimit = min(msg.GasLimit, maxPredecessorGas)
		txConfig.TxHash = ethTx.Hash()
		txConfig.TxIndex = uint(i) //nolint:gosec // G115 // won't exceed uint64

		ctx = buildTraceCtx(ctx, msg.GasLimit)
		// we ignore the error here. this endpoint, ideally, is called internally from the ETH backend, which will call this query
		// using all previous txs in the trace transaction's block. some of those _could_ be invalid transactions.
		rsp, _ := k.ApplyMessageWithConfig(ctx, *msg, nil, true, cfg, txConfig, false)
		if rsp != nil {
			ctx.GasMeter().ConsumeGas(rsp.GasUsed, "evm predecessor tx")
			txConfig.LogIndex += uint(len(rsp.Logs))
		}
	}

	tx := req.Msg.AsTransaction()
	txConfig.TxHash = tx.Hash()
	if len(req.Predecessors) > 0 {
		txConfig.TxIndex++
	}

	result, _, err := k.traceTx(ctx, cfg, txConfig, signer, common.BytesToAddress(req.Msg.From), tx, req.TraceConfig, false)
	if err != nil {
		// error will be returned with detail status from traceTx
		return nil, err
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceTxResponse{
		Data: resultData,
	}, nil
}

// TraceBlock configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment for all the transactions in the queried block.
// The return value will be tracer dependent.
func (k Keeper) TraceBlock(c context.Context, req *types.QueryTraceBlockRequest) (*types.QueryTraceBlockResponse, error) {
	fmt.Println("trace block 1")
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	fmt.Println("trace block 2")

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	// get the context of block beginning
	contextHeight := req.BlockNumber
	if contextHeight < 1 {
		// 0 is a special value in `ContextWithHeight`
		contextHeight = 1
	}

	fmt.Println("trace block 3")

	ctx := sdk.UnwrapSDKContext(c)
	ctx = ctx.WithBlockHeight(contextHeight)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	fmt.Println("trace block 4")

	// to get the base fee we only need the block max gas in the consensus params
	ctx = ctx.WithConsensusParams(tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{MaxGas: req.BlockMaxGas},
	})

	fmt.Println("trace block 5")

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load evm config")
	}

	// compute and use base fee of height that is being traced
	baseFee := k.feeMarketWrapper.CalculateBaseFee(ctx)
	if baseFee != nil {
		cfg.BaseFee = baseFee
	}

	fmt.Println("trace block 6")

	signer := ethtypes.MakeSigner(types.GetEthChainConfig(), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	txsLength := len(req.Txs)
	results := make([]*types.TxTraceResult, 0, txsLength)

	txConfig := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash()))

	fmt.Println("trace block 7")

	for i, tx := range req.Txs {
		result := types.TxTraceResult{}
		ethTx := tx.AsTransaction()
		txConfig.TxHash = ethTx.Hash()
		txConfig.TxIndex = uint(i) //nolint:gosec // G115 // won't exceed uint64
		fmt.Println("trace block 8")

		traceResult, logIndex, err := k.traceTx(ctx, cfg, txConfig, signer, common.BytesToAddress(tx.From), ethTx, req.TraceConfig, true)
		fmt.Println("trace block 9")

		if err != nil {
			result.Error = err.Error()
		} else {
			txConfig.LogIndex = logIndex
			result.Result = traceResult
		}
		results = append(results, &result)
	}

	fmt.Println("trace block 10")

	resultData, err := json.Marshal(results)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	fmt.Println("trace block 11")

	return &types.QueryTraceBlockResponse{
		Data: resultData,
	}, nil
}

// traceTx do trace on one transaction, it returns a tuple: (traceResult, nextLogIndex, error).
func (k *Keeper) traceTx(
	ctx sdk.Context,
	cfg *statedb.EVMConfig,
	txConfig statedb.TxConfig,
	signer ethtypes.Signer,
	from common.Address,
	tx *ethtypes.Transaction,
	traceConfig *types.TraceConfig,
	commitMessage bool,
) (*any, uint, error) {
	fmt.Println("trace tx1")
	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer           *tracers.Tracer
		overrides        *ethparams.ChainConfig
		jsonTracerConfig json.RawMessage
		err              error
		timeout          = defaultTraceTimeout
	)

	fmt.Println("trace tx2")

	var msg *core.Message
	if !isUnsigned(tx) {
		fmt.Println("trace tx3")

		msg, err = core.TransactionToMessage(tx, signer, cfg.BaseFee)
		if err != nil {
			return nil, 0, status.Errorf(codes.InvalidArgument, "transaction to message: %v", err.Error())
		}
		fmt.Println("trace tx5")

	} else {
		fmt.Println("trace tx4")

		msg = unsignedTxAsMessage(from, tx, cfg.BaseFee)

		fmt.Println("trace tx6")

	}

	if traceConfig == nil {
		traceConfig = &types.TraceConfig{}
	}

	fmt.Println("trace tx7")

	if traceConfig != nil && traceConfig.TracerJsonConfig != "" {
		// ignore error. default to no traceConfig
		_ = json.Unmarshal([]byte(traceConfig.TracerJsonConfig), &jsonTracerConfig)
	}

	fmt.Println("trace tx8")

	if traceConfig.Overrides != nil {
		overrides = traceConfig.Overrides.EthereumConfig(types.GetEthChainConfig().ChainID)
	}

	logConfig := logger.Config{
		EnableMemory:     traceConfig.EnableMemory,
		DisableStorage:   traceConfig.DisableStorage,
		DisableStack:     traceConfig.DisableStack,
		EnableReturnData: traceConfig.EnableReturnData,
		Limit:            int(traceConfig.Limit),
		Overrides:        overrides,
	}

	fmt.Println("trace tx9")

	sLogger := logger.NewStructLogger(&logConfig)
	tracer = &tracers.Tracer{
		Hooks:     sLogger.Hooks(),
		GetResult: sLogger.GetResult,
		Stop:      sLogger.Stop,
	}

	fmt.Println("trace tx10")

	tCtx := &tracers.Context{
		BlockHash: txConfig.BlockHash,
		TxIndex:   int(txConfig.TxIndex), //#nosec G115 -- int overflow is not a concern here
		TxHash:    txConfig.TxHash,
	}

	fmt.Println("trace tx11")

	if traceConfig.Tracer != "" {
		if tracer, err = tracers.DefaultDirectory.New(traceConfig.Tracer, tCtx, jsonTracerConfig,
			types.GetEthChainConfig()); err != nil {
			return nil, 0, status.Error(codes.Internal, err.Error())
		}
	}

	fmt.Println("trace tx12")

	// Define a meaningful timeout of a single transaction trace
	if traceConfig.Timeout != "" {
		if timeout, err = time.ParseDuration(traceConfig.Timeout); err != nil {
			return nil, 0, status.Errorf(codes.InvalidArgument, "timeout value: %s", err.Error())
		}
	}

	// Handle timeouts and RPC cancellations
	deadlineCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
	defer cancel()

	go func() {
		<-deadlineCtx.Done()
		if errors.Is(deadlineCtx.Err(), context.DeadlineExceeded) {
			tracer.Stop(errors.New("execution timeout"))
		}
	}()

	fmt.Println("trace tx13")

	// Build EVM execution context
	ctx = buildTraceCtx(ctx, msg.GasLimit)
	res, err := k.ApplyMessageWithConfig(ctx, *msg, tracer.Hooks, commitMessage, cfg, txConfig, false)
	if err != nil {
		return nil, 0, status.Error(codes.Internal, err.Error())
	}

	fmt.Println("trace tx14")

	var result interface{}
	result, err = tracer.GetResult()
	if err != nil {
		return nil, 0, status.Error(codes.Internal, err.Error())
	}

	fmt.Println("trace tx15")

	return &result, txConfig.LogIndex + uint(len(res.Logs)), nil
}

// BaseFee implements the Query/BaseFee gRPC method
func (k Keeper) BaseFee(c context.Context, _ *types.QueryBaseFeeRequest) (*types.QueryBaseFeeResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	baseFee := k.GetBaseFee(ctx)

	res := &types.QueryBaseFeeResponse{}
	if baseFee != nil {
		aux := sdkmath.NewIntFromBigInt(baseFee)
		res.BaseFee = &aux
	}

	return res, nil
}

// GlobalMinGasPrice implements the Query/GlobalMinGasPrice gRPC method
func (k Keeper) GlobalMinGasPrice(c context.Context, _ *types.QueryGlobalMinGasPriceRequest) (*types.QueryGlobalMinGasPriceResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	minGasPrice := k.GetMinGasPrice(ctx).TruncateInt()
	return &types.QueryGlobalMinGasPriceResponse{MinGasPrice: minGasPrice}, nil
}

// Config implements the Query/Config gRPC method
func (k Keeper) Config(_ context.Context, _ *types.QueryConfigRequest) (*types.QueryConfigResponse, error) {
	config := types.GetChainConfig()
	config.Denom = types.GetEVMCoinDenom()
	config.Decimals = uint64(types.GetEVMCoinDecimals())

	return &types.QueryConfigResponse{Config: config}, nil
}

// buildTraceCtx builds a context for simulating or tracing transactions by:
// 1. assigning a new infinite gas meter with the provided gasLimit
// 2. calling BuildEvmExecutionCtx to set up gas configs consistent with Ethereum transaction execution.
func buildTraceCtx(ctx sdk.Context, gasLimit uint64) sdk.Context {
	return evmante.BuildEvmExecutionCtx(ctx).
		WithGasMeter(cosmosevmtypes.NewInfiniteGasMeterWithLimit(gasLimit))
}

func isUnsigned(ethTx *ethtypes.Transaction) bool {
	r, v, s := ethTx.RawSignatureValues()

	return (r == nil && v == nil && s == nil) || (r.Int64() == 0 && v.Int64() == 0 && s.Int64() == 0)
}

// same as core.TransactionToMessage, just from is provided instead of calculated from signature
func unsignedTxAsMessage(from common.Address, tx *ethtypes.Transaction, baseFee *big.Int) *core.Message {
	msg := &core.Message{
		From:                  from,
		Nonce:                 tx.Nonce(),
		GasLimit:              tx.Gas(),
		GasPrice:              new(big.Int).Set(tx.GasPrice()),
		GasFeeCap:             new(big.Int).Set(tx.GasFeeCap()),
		GasTipCap:             new(big.Int).Set(tx.GasTipCap()),
		To:                    tx.To(),
		Value:                 tx.Value(),
		Data:                  tx.Data(),
		AccessList:            tx.AccessList(),
		SetCodeAuthorizations: tx.SetCodeAuthorizations(),
		SkipNonceChecks:       false,
		SkipFromEOACheck:      false,
		BlobHashes:            tx.BlobHashes(),
		BlobGasFeeCap:         tx.BlobGasFeeCap(),
	}
	// If baseFee provided, set gasPrice to effectiveGasPrice.
	if baseFee != nil {
		msg.GasPrice = msg.GasPrice.Add(msg.GasTipCap, baseFee)
		if msg.GasPrice.Cmp(msg.GasFeeCap) > 0 {
			msg.GasPrice = msg.GasFeeCap
		}
	}

	return msg
}
