package keeper

import (
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var isTrue = []byte("0x01")

const addressLength = 42

// GetParams returns the total set of erc20 parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	enableErc20 := k.IsERC20Enabled(ctx)
	dynamicPrecompiles := k.getDynamicPrecompiles(ctx)
	nativePrecompiles := k.getNativePrecompiles(ctx)
	permissionlessRegistration := k.isPermissionlessRegistration(ctx)
	return types.NewParams(enableErc20, nativePrecompiles, dynamicPrecompiles, permissionlessRegistration)
}

// UpdateCodeHash takes in the updated parameters and
// compares the new set of native and dynamic precompiles to the current
// parameter set.
//
// If there is a diff, the ERC-20 code hash for all precompiles that are removed from the list
// will be removed from the store. Meanwhile, for all newly added precompiles the code hash will be
// registered.
func (k Keeper) UpdateCodeHash(ctx sdk.Context, newParams types.Params) error {
	oldNativePrecompiles := k.getNativePrecompiles(ctx)
	oldDynamicPrecompiles := k.getDynamicPrecompiles(ctx)

	if err := k.RegisterOrUnregisterERC20CodeHashes(ctx, oldDynamicPrecompiles, newParams.DynamicPrecompiles); err != nil {
		return err
	}

	return k.RegisterOrUnregisterERC20CodeHashes(ctx, oldNativePrecompiles, newParams.NativePrecompiles)
}

// RegisterOrUnregisterERC20CodeHashes takes two arrays of precompiles as its argument:
//   - previously registered precompiles
//   - new set of precompiles to be registered
//
// It then compares the two arrays and registers the code hash for all precompiles that are newly added
// and unregisters the code hash for all precompiles that are removed from the list.
func (k Keeper) RegisterOrUnregisterERC20CodeHashes(ctx sdk.Context, oldPrecompiles, newPrecompiles []string) error {
	for _, precompile := range oldPrecompiles {
		if slices.Contains(newPrecompiles, precompile) {
			continue
		}

		if err := k.UnRegisterERC20CodeHash(ctx, common.HexToAddress(precompile)); err != nil {
			return err
		}
	}

	for _, precompile := range newPrecompiles {
		if slices.Contains(oldPrecompiles, precompile) {
			continue
		}

		if err := k.RegisterERC20CodeHash(ctx, common.HexToAddress(precompile)); err != nil {
			return err
		}
	}

	return nil
}

// SetParams sets the erc20 parameters to the param space.
func (k Keeper) SetParams(ctx sdk.Context, newParams types.Params) error {
	if err := newParams.Validate(); err != nil {
		return err
	}

	if err := k.UpdateCodeHash(ctx, newParams); err != nil {
		return err
	}

	k.setERC20Enabled(ctx, newParams.EnableErc20)
	k.setDynamicPrecompiles(ctx, newParams.DynamicPrecompiles)
	k.setNativePrecompiles(ctx, newParams.NativePrecompiles)
	k.SetPermissionlessRegistration(ctx, newParams.PermissionlessRegistration)
	return nil
}

// IsERC20Enabled returns true if the module logic is enabled
func (k Keeper) IsERC20Enabled(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyEnableErc20)
}

// setERC20Enabled sets the EnableERC20 param in the store
func (k Keeper) setERC20Enabled(ctx sdk.Context, enable bool) {
	store := ctx.KVStore(k.storeKey)
	if enable {
		store.Set(types.ParamStoreKeyEnableErc20, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyEnableErc20)
}

// setDynamicPrecompiles sets the DynamicPrecompiles KVStore
func (k Keeper) setDynamicPrecompiles(ctx sdk.Context, dynamicPrecompiles map[string]byte) {
	store := ctx.KVStore(types.StoreKeyDynamicPrecompiles))
	for np, _ := range dynamicPrecompiles {
		store.Set(np, nil)
	}
}

// getDynamicPrecompiles returns the DynamicPrecompiles KVStore
func (k Keeper) getDynamicPrecompiles(ctx sdk.Context) map[string]bool {
	return ctx.KVStore(types.StoreKeyDynamicPrecompiles)
}

// setNativePrecompiles sets the NativePrecompiles KVStore
func (k Keeper) setNativePrecompiles(ctx sdk.Context, nativePrecompiles map[string]bool) {
	store := ctx.KVStore(types.StoreKeyNativePrecompiles)
	for np, _ := range nativePrecompiles {
		store.Set(np, nil)
	}
}

// getNativePrecompiles returns the NativePrecompiles KVStore
func (k Keeper) getNativePrecompiles(ctx sdk.Context) map[string]bool {
	return ctx.KVStore(types.StoreKeyNativePrecompiles)
}

// isPermissionlessRegistration returns true if the module enabled permissionless
// erc20 registration
func (k Keeper) isPermissionlessRegistration(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyPermissionlessRegistration)
}

func (k Keeper) SetPermissionlessRegistration(ctx sdk.Context, permissionlessRegistration bool) {
	store := ctx.KVStore(k.storeKey)
	if permissionlessRegistration {
		store.Set(types.ParamStoreKeyPermissionlessRegistration, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyPermissionlessRegistration)
}
