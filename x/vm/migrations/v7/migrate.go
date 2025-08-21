package v7

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type evmKeeper interface {
	SetCodeHash(ctx sdk.Context, addrBytes, hashBytes []byte)
	GetParams(ctx sdk.Context) (params evmtypes.Params)
	SetParams(ctx sdk.Context, params evmtypes.Params) error
}

func MigrateStore(ctx sdk.Context, ek evmKeeper, ak evmtypes.AccountKeeper) error {
	ctx.Logger().Info("migrating store from consensus version 6 to 7")
	return nil
}
