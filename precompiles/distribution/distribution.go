package distribution

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"

	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for distribution.
type Precompile struct {
	cmn.Precompile
	distributionKeeper distributionkeeper.Keeper
	stakingKeeper      stakingkeeper.Keeper
	evmKeeper          *evmkeeper.Keeper
	addrCdc            address.Codec
}

// NewPrecompile creates a new distribution Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	distributionKeeper distributionkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	stakingKeeper stakingkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
	addrCdc address.Codec,
) (*Precompile, error) {
	newAbi, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, fmt.Errorf("error loading the distribution ABI %s", err)
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                   newAbi,
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		evmKeeper:          evmKeeper,
		addrCdc:            addrCdc,
	}

	// SetAddress defines the address of the distribution compile contract.
	p.SetAddress(common.HexToAddress(evmtypes.DistributionPrecompileAddress))

	return p, nil
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// TODO: refactor this to be used in the common precompile method on a separate PR

	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run executes the precompiled contract distribution methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	bz, err = p.run(evm, contract, readOnly)
	if err != nil {
		return cmn.ReturnRevertError(evm, err)
	}

	return bz, nil
}

func (p Precompile) run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	// Start the balance change handler before executing the precompile.
	var balanceHandler *cmn.BalanceHandler
	if p.BalanceHandlerFactory != nil {
		balanceHandler = p.BalanceHandlerFactory.NewBalanceHandler()
	}

	if balanceHandler != nil {
		balanceHandler.BeforeBalanceChange(ctx)
	}

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	// Custom transactions
	case ClaimRewardsMethod:
		bz, err = p.ClaimRewards(ctx, contract, stateDB, method, args)
	// Distribution transactions
	case SetWithdrawAddressMethod:
		bz, err = p.SetWithdrawAddress(ctx, contract, stateDB, method, args)
	case WithdrawDelegatorRewardMethod:
		bz, err = p.WithdrawDelegatorReward(ctx, contract, stateDB, method, args)
	case WithdrawValidatorCommissionMethod:
		bz, err = p.WithdrawValidatorCommission(ctx, contract, stateDB, method, args)
	case FundCommunityPoolMethod:
		bz, err = p.FundCommunityPool(ctx, contract, stateDB, method, args)
	case DepositValidatorRewardsPoolMethod:
		bz, err = p.DepositValidatorRewardsPool(ctx, contract, stateDB, method, args)
	// Distribution queries
	case ValidatorDistributionInfoMethod:
		bz, err = p.ValidatorDistributionInfo(ctx, contract, method, args)
	case ValidatorOutstandingRewardsMethod:
		bz, err = p.ValidatorOutstandingRewards(ctx, contract, method, args)
	case ValidatorCommissionMethod:
		bz, err = p.ValidatorCommission(ctx, contract, method, args)
	case ValidatorSlashesMethod:
		bz, err = p.ValidatorSlashes(ctx, contract, method, args)
	case DelegationRewardsMethod:
		bz, err = p.DelegationRewards(ctx, contract, method, args)
	case DelegationTotalRewardsMethod:
		bz, err = p.DelegationTotalRewards(ctx, contract, method, args)
	case DelegatorValidatorsMethod:
		bz, err = p.DelegatorValidators(ctx, contract, method, args)
	case DelegatorWithdrawAddressMethod:
		bz, err = p.DelegatorWithdrawAddress(ctx, contract, method, args)
	case CommunityPoolMethod:
		bz, err = p.CommunityPool(ctx, contract, method, args)
	}

	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	// Process the native balance changes after the method execution.
	if balanceHandler != nil {
		if err := balanceHandler.AfterBalanceChange(ctx, stateDB); err != nil {
			return nil, err
		}
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available distribution transactions are:
//   - ClaimRewards
//   - SetWithdrawAddress
//   - WithdrawDelegatorReward
//   - WithdrawValidatorCommission
//   - FundCommunityPool
//   - DepositValidatorRewardsPool
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case ClaimRewardsMethod,
		SetWithdrawAddressMethod,
		WithdrawDelegatorRewardMethod,
		WithdrawValidatorCommissionMethod,
		FundCommunityPoolMethod,
		DepositValidatorRewardsPoolMethod:
		return true
	default:
		return false
	}
}
