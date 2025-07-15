package v6

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type evmKeeper interface {
}

func MigrateStore(ctx sdk.Context, observerKeeper evmKeeper) error {
	fmt.Println("MigrateStore V5->V6")

	return nil
}
