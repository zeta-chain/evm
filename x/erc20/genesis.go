package erc20

import (
	"fmt"
	"maps"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
)

// InitGenesis import module genesis
func InitGenesis(
	ctx sdk.Context,
	k keeper.Keeper,
	accountKeeper authkeeper.AccountKeeper,
	data types.GenesisState,
) {
	err := k.SetParams(ctx, data.Params)
	if err != nil {
		panic(fmt.Errorf("error setting params %s", err))
	}

	// ensure erc20 module account is set on genesis
	if acc := accountKeeper.GetModuleAccount(ctx, types.ModuleName); acc == nil {
		// NOTE: shouldn't occur
		panic("the erc20 module account has not been set")
	}

	for _, pair := range data.TokenPairs {
		k.SetToken(ctx, pair)
	}

	for _, precompile := range data.NativePrecompiles {
		addr := common.HexToAddress(precompile)
		if err := k.RegisterCodeHash(ctx, addr, keeper.PrecompileTypeNative); err != nil {
			panic(fmt.Errorf("error registering native precompiles %s", err))
		}
		k.SetNativePrecompile(ctx, addr)
	}
	for _, precompile := range data.DynamicPrecompiles {
		addr := common.HexToAddress(precompile)
		if err := k.RegisterCodeHash(ctx, addr, keeper.PrecompileTypeDynamic); err != nil {
			panic(fmt.Errorf("error registering dynamic precompiles %s", err))
		}
		k.SetDynamicPrecompile(ctx, addr)
	}

	for _, allowance := range data.Allowances {
		erc20 := common.HexToAddress(allowance.Erc20Address)
		owner := common.HexToAddress(allowance.Owner)
		spender := common.HexToAddress(allowance.Spender)
		value := allowance.Value.BigInt()
		err := k.UnsafeSetAllowance(ctx, erc20, owner, spender, value)
		if err != nil {
			panic(fmt.Errorf("error setting allowance %s", err))
		}
	}
}

// ExportGenesis export module status
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	nps := slices.Sorted(maps.Keys(k.GetNativePrecompiles(ctx)))
	dps := slices.Sorted(maps.Keys(k.GetDynamicPrecompiles(ctx)))

	return &types.GenesisState{
		Params:             k.GetParams(ctx),
		TokenPairs:         k.GetTokenPairs(ctx),
		Allowances:         k.GetAllowances(ctx),
		NativePrecompiles:  nps,
		DynamicPrecompiles: dps,
	}
}
