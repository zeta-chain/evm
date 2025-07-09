package keeper

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/werc20"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PrecompileType int

const (
	PrecompileTypeNative PrecompileType = iota
	PrecompileTypeDynamic
)

// GetERC20PrecompileInstance returns the precompile instance for the given address.
func (k Keeper) GetERC20PrecompileInstance(
	ctx sdk.Context,
	address common.Address,
) (contract vm.PrecompiledContract, found bool, err error) {
	isNative := k.IsNativePrecompileAvailable(ctx, address)
	isDynamic := k.IsDynamicPrecompileAvailable(ctx, address)

	if available := isNative || isDynamic; !available {
		return nil, false, nil
	}

	precompile, err := k.InstantiateERC20Precompile(ctx, address, isNative)
	if err != nil {
		return nil, false, errorsmod.Wrapf(err, "precompiled contract not initialized: %s", address.String())
	}

	return precompile, true, nil
}

// InstantiateERC20Precompile returns an ERC20 precompile instance for the given
// contract address.
// If the `hasWrappedMethods` boolean is true, the ERC20 instance returned
// exposes methods for `withdraw` and `deposit` as it is common for wrapped tokens.
func (k Keeper) InstantiateERC20Precompile(ctx sdk.Context, contractAddr common.Address, hasWrappedMethods bool) (vm.PrecompiledContract, error) {
	address := contractAddr.String()
	// check if the precompile is an ERC20 contract
	id := k.GetTokenPairID(ctx, address)
	if len(id) == 0 {
		return nil, fmt.Errorf("precompile id not found: %s", address)
	}
	pair, ok := k.GetTokenPair(ctx, id)
	if !ok {
		return nil, fmt.Errorf("token pair not found: %s", address)
	}

	if hasWrappedMethods {
		return werc20.NewPrecompile(pair, k.bankKeeper, k, *k.transferKeeper)
	}

	return erc20.NewPrecompile(pair, k.bankKeeper, k, *k.transferKeeper)
}

// RegisterCodeHash checks if a new precompile already exists and registers the code hash it is not
func (k Keeper) RegisterCodeHash(ctx sdk.Context, addr common.Address, pType PrecompileType) error {
	shouldRegister := false
	switch pType {
	case PrecompileTypeNative:
		shouldRegister = !k.IsNativePrecompileAvailable(ctx, addr)
	case PrecompileTypeDynamic:
		shouldRegister = !k.IsDynamicPrecompileAvailable(ctx, addr)
	default:
		return fmt.Errorf("invalid precompile type: %v", pType)
	}

	if shouldRegister {
		if err := k.RegisterERC20CodeHash(ctx, addr); err != nil {
			return err
		}
	}

	return nil
}

// EnableNativePrecompile adds the address of the given precompile to the prefix store
func (k Keeper) EnableNativePrecompile(ctx sdk.Context, addr common.Address) error {
	k.Logger(ctx).Info("Added new precompiles", "addresses", addr)
	if err := k.RegisterCodeHash(ctx, addr, PrecompileTypeNative); err != nil {
		return err
	}
	k.SetNativePrecompile(ctx, addr)
	return nil
}
