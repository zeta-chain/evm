package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper grants access to the EVM module state and implements the go-ethereum StateDB interface.
type Keeper struct {
	// Protobuf codec
	cdc codec.BinaryCodec
	// Store key required for the EVM Prefix KVStore. It is required by:
	// - storing account's Storage State
	// - storing account's Code
	// - storing transaction Logs
	// - storing Bloom filters by block height. Needed for the Web3 API.
	storeKey storetypes.StoreKey

	// key to access the transient store, which is reset on every block during Commit
	transientKey storetypes.StoreKey

	// KVStore Keys for modules wired to app
	storeKeys map[string]*storetypes.KVStoreKey

	// the address capable of executing a MsgUpdateParams message. Typically, this should be the x/gov module account.
	authority sdk.AccAddress

	// access to account state
	accountKeeper types.AccountKeeper

	// bankWrapper is used to convert the Cosmos SDK coin used in the EVM to the
	// proper decimal representation.
	bankWrapper types.BankWrapper

	// access historical headers for EVM state transition execution
	stakingKeeper types.StakingKeeper
	// fetch EIP1559 base fee and parameters
	feeMarketWrapper *wrappers.FeeMarketWrapper
	// erc20Keeper interface needed to instantiate erc20 precompiles
	erc20Keeper types.Erc20Keeper
	// consensusKeeper is used to get consensus params during query contexts.
	// This is needed as block.gasLimit is expected to be available in eth_call, which is routed through Cosmos SDK's
	// grpc query router. This query router builds a context WITHOUT consensus params, so we manually supply the context
	// with consensus params when not set in context.
	consensusKeeper types.ConsensusParamsKeeper

	// Tracer used to collect execution traces from the EVM transaction execution
	tracer string

	hooks types.EvmHooks
	// EVM Hooks for tx post-processing

	// precompiles defines the map of all available precompiled smart contracts.
	// Some of these precompiled contracts might not be active depending on the EVM
	// parameters.
	precompiles map[common.Address]vm.PrecompiledContract

	// evmMempool is the custom EVM appside mempool
	// if it is nil, the default comet mempool will be used
	evmMempool *evmmempool.ExperimentalEVMMempool
}

// NewKeeper generates new evm module keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey, transientKey storetypes.StoreKey,
	keys map[string]*storetypes.KVStoreKey,
	authority sdk.AccAddress,
	ak types.AccountKeeper,
	bankKeeper types.BankKeeper,
	sk types.StakingKeeper,
	fmk types.FeeMarketKeeper,
	consensusKeeper types.ConsensusParamsKeeper,
	erc20Keeper types.Erc20Keeper,
	tracer string,
) *Keeper {
	// ensure evm module account is set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the EVM module account has not been set")
	}

	// ensure the authority account is correct
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	bankWrapper := wrappers.NewBankWrapper(bankKeeper)
	feeMarketWrapper := wrappers.NewFeeMarketWrapper(fmk)

	// NOTE: we pass in the parameter space to the CommitStateDB in order to use custom denominations for the EVM operations
	return &Keeper{
		cdc:              cdc,
		authority:        authority,
		accountKeeper:    ak,
		bankWrapper:      bankWrapper,
		stakingKeeper:    sk,
		feeMarketWrapper: feeMarketWrapper,
		storeKey:         storeKey,
		transientKey:     transientKey,
		tracer:           tracer,
		consensusKeeper:  consensusKeeper,
		erc20Keeper:      erc20Keeper,
		storeKeys:        keys,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", types.ModuleName)
}

// ----------------------------------------------------------------------------
// Block Bloom
// Required by Web3 API.
// ----------------------------------------------------------------------------

// EmitBlockBloomEvent emit block bloom events
func (k Keeper) EmitBlockBloomEvent(ctx sdk.Context, bloom ethtypes.Bloom) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBlockBloom,
			sdk.NewAttribute(types.AttributeKeyEthereumBloom, string(bloom.Bytes())),
		),
	)
}

// GetAuthority returns the x/evm module authority address
func (k Keeper) GetAuthority() sdk.AccAddress {
	return k.authority
}

// GetBlockBloomTransient returns bloom bytes for the current block height
func (k Keeper) GetBlockBloomTransient(ctx sdk.Context) *big.Int {
	store := prefix.NewStore(ctx.TransientStore(k.transientKey), types.KeyPrefixTransientBloom)
	heightBz := sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight())) //nolint:gosec // G115 // won't exceed uint64
	bz := store.Get(heightBz)
	if len(bz) == 0 {
		return big.NewInt(0)
	}

	return new(big.Int).SetBytes(bz)
}

// SetBlockBloomTransient sets the given bloom bytes to the transient store. This value is reset on
// every block.
func (k Keeper) SetBlockBloomTransient(ctx sdk.Context, bloom *big.Int) {
	store := prefix.NewStore(ctx.TransientStore(k.transientKey), types.KeyPrefixTransientBloom)
	heightBz := sdk.Uint64ToBigEndian(uint64(ctx.BlockHeight())) //nolint:gosec // G115 // won't exceed uint64
	store.Set(heightBz, bloom.Bytes())
}

// ----------------------------------------------------------------------------
// Tx
// ----------------------------------------------------------------------------

// SetTxIndexTransient set the index of processing transaction
func (k Keeper) SetTxIndexTransient(ctx sdk.Context, index uint64) {
	store := ctx.TransientStore(k.transientKey)
	store.Set(types.KeyPrefixTransientTxIndex, sdk.Uint64ToBigEndian(index))
}

// GetTxIndexTransient returns EVM transaction index on the current block.
func (k Keeper) GetTxIndexTransient(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.transientKey)
	return sdk.BigEndianToUint64(store.Get(types.KeyPrefixTransientTxIndex))
}

// ----------------------------------------------------------------------------
// Hooks
// ----------------------------------------------------------------------------

// SetHooks sets the hooks for the EVM module
// Called only once during initialization, panics if called more than once.
func (k *Keeper) SetHooks(eh types.EvmHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set evm hooks twice")
	}

	k.hooks = eh
	return k
}

// PostTxProcessing delegates the call to the hooks.
// If no hook has been registered, this function returns with a `nil` error
func (k *Keeper) PostTxProcessing(
	ctx sdk.Context,
	sender common.Address,
	msg core.Message,
	receipt *ethtypes.Receipt,
) error {
	if k.hooks == nil {
		return nil
	}
	return k.hooks.PostTxProcessing(ctx, sender, msg, receipt)
}

// ----------------------------------------------------------------------------
// Log
// ----------------------------------------------------------------------------

// GetLogSizeTransient returns EVM log index on the current block.
func (k Keeper) GetLogSizeTransient(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.transientKey)
	return sdk.BigEndianToUint64(store.Get(types.KeyPrefixTransientLogSize))
}

// SetLogSizeTransient fetches the current EVM log index from the transient store, increases its
// value by one and then sets the new index back to the transient store.
func (k Keeper) SetLogSizeTransient(ctx sdk.Context, logSize uint64) {
	store := ctx.TransientStore(k.transientKey)
	store.Set(types.KeyPrefixTransientLogSize, sdk.Uint64ToBigEndian(logSize))
}

// ----------------------------------------------------------------------------
// Storage
// ----------------------------------------------------------------------------

// GetAccountStorage return state storage associated with an account
func (k Keeper) GetAccountStorage(ctx sdk.Context, address common.Address) types.Storage {
	storage := types.Storage{}

	k.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return true
	})

	return storage
}

// ----------------------------------------------------------------------------
// Account
// ----------------------------------------------------------------------------

// Tracer return a default vm.Tracer based on current keeper state
func (k Keeper) Tracer(ctx sdk.Context, msg core.Message, ethCfg *params.ChainConfig) *tracing.Hooks {
	return types.NewTracer(k.tracer, msg, ethCfg, ctx.BlockHeight(), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
}

// GetAccountWithoutBalance load nonce and codehash without balance,
// more efficient in cases where balance is not needed.
func (k *Keeper) GetAccountWithoutBalance(ctx sdk.Context, addr common.Address) *statedb.Account {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		return nil
	}

	codeHashBz := k.GetCodeHash(ctx, addr).Bytes()

	return &statedb.Account{
		Nonce:    acct.GetSequence(),
		CodeHash: codeHashBz,
	}
}

// GetAccountOrEmpty returns empty account if not exist.
func (k *Keeper) GetAccountOrEmpty(ctx sdk.Context, addr common.Address) statedb.Account {
	acct := k.GetAccount(ctx, addr)
	if acct != nil {
		return *acct
	}

	// empty account
	return statedb.Account{
		Balance:  new(uint256.Int),
		CodeHash: types.EmptyCodeHash,
	}
}

// GetNonce returns the sequence number of an account, returns 0 if not exists.
func (k *Keeper) GetNonce(ctx sdk.Context, addr common.Address) uint64 {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		return 0
	}

	return acct.GetSequence()
}

// SpendableCoin load account's balance of gas token.
func (k *Keeper) SpendableCoin(ctx sdk.Context, addr common.Address) *uint256.Int {
	cosmosAddr := sdk.AccAddress(addr.Bytes())

	// Get the balance via bank wrapper to convert it to 18 decimals if needed.
	coin := k.bankWrapper.SpendableCoin(ctx, cosmosAddr, types.GetEVMCoinDenom())

	result, err := utils.Uint256FromBigInt(coin.Amount.BigInt())
	if err != nil {
		return nil
	}

	return result
}

// GetBalance load account's balance of gas token.
func (k *Keeper) GetBalance(ctx sdk.Context, addr common.Address) *uint256.Int {
	cosmosAddr := sdk.AccAddress(addr.Bytes())

	// Get the balance via bank wrapper to convert it to 18 decimals if needed.
	coin := k.bankWrapper.GetBalance(ctx, cosmosAddr, types.GetEVMCoinDenom())

	result, err := utils.Uint256FromBigInt(coin.Amount.BigInt())
	if err != nil {
		return nil
	}

	return result
}

// GetBaseFee returns current base fee, return values:
// - `nil`: london hardfork not enabled.
// - `0`: london hardfork enabled but feemarket is not enabled.
// - `n`: both london hardfork and feemarket are enabled.
func (k Keeper) GetBaseFee(ctx sdk.Context) *big.Int {
	ethCfg := types.GetEthChainConfig()
	if !types.IsLondon(ethCfg, ctx.BlockHeight()) {
		return nil
	}
	baseFee := k.feeMarketWrapper.GetBaseFee(ctx)
	if baseFee == nil {
		// return 0 if feemarket not enabled.
		baseFee = big.NewInt(0)
	}
	return baseFee
}

// GetMinGasMultiplier returns the MinGasMultiplier param from the fee market module
func (k Keeper) GetMinGasMultiplier(ctx sdk.Context) math.LegacyDec {
	return k.feeMarketWrapper.GetParams(ctx).MinGasMultiplier
}

// GetMinGasPrice returns the MinGasPrice param from the fee market module
// adapted according to the evm denom decimals
func (k Keeper) GetMinGasPrice(ctx sdk.Context) math.LegacyDec {
	return k.feeMarketWrapper.GetParams(ctx).MinGasPrice
}

// ResetTransientGasUsed reset gas used to prepare for execution of current cosmos tx, called in ante handler.
func (k Keeper) ResetTransientGasUsed(ctx sdk.Context) {
	store := ctx.TransientStore(k.transientKey)
	store.Delete(types.KeyPrefixTransientGasUsed)
}

// GetTransientGasUsed returns the gas used by current cosmos tx.
func (k Keeper) GetTransientGasUsed(ctx sdk.Context) uint64 {
	store := ctx.TransientStore(k.transientKey)
	return sdk.BigEndianToUint64(store.Get(types.KeyPrefixTransientGasUsed))
}

// SetTransientGasUsed sets the gas used by current cosmos tx.
func (k Keeper) SetTransientGasUsed(ctx sdk.Context, gasUsed uint64) {
	store := ctx.TransientStore(k.transientKey)
	bz := sdk.Uint64ToBigEndian(gasUsed)
	store.Set(types.KeyPrefixTransientGasUsed, bz)
}

// AddTransientGasUsed accumulate gas used by each eth msgs included in current cosmos tx.
func (k Keeper) AddTransientGasUsed(ctx sdk.Context, gasUsed uint64) (uint64, error) {
	result := k.GetTransientGasUsed(ctx) + gasUsed
	if result < gasUsed {
		return 0, errorsmod.Wrap(types.ErrGasOverflow, "transient gas used")
	}
	k.SetTransientGasUsed(ctx, result)
	return result, nil
}

// KVStoreKeys returns KVStore keys injected to keeper
func (k Keeper) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return k.storeKeys
}

// SetEvmMempool sets the evm mempool
func (k *Keeper) SetEvmMempool(evmMempool *evmmempool.ExperimentalEVMMempool) {
	k.evmMempool = evmMempool
}

// GetEvmMempool returns the evm mempool
func (k Keeper) GetEvmMempool() *evmmempool.ExperimentalEVMMempool {
	return k.evmMempool
}
