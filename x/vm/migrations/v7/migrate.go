package v7

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type evmKeeper interface {
	GetParams(ctx sdk.Context) (params evmtypes.Params)
	SetParams(ctx sdk.Context, params evmtypes.Params) error
}

func MigrateStore(ctx sdk.Context, ek evmKeeper) error {
	params := ek.GetParams(ctx)
	if len(params.ActiveStaticPrecompiles) == 0 {
		params.ActiveStaticPrecompiles = []string{
			"0x0000000000000000000000000000000000000800",
			"0x0000000000000000000000000000000000000806",
		}
	}

	err := ek.SetParams(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to set params: %w", err)
	}
	return nil
}
