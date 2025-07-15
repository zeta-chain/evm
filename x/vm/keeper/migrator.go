package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	v6 "github.com/cosmos/evm/x/vm/migrations/v6"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	evmKeeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{
		evmKeeper: keeper,
	}
}

// Migrate9to10 migrates the store from consensus version 5 to 6
func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	return v6.MigrateStore(ctx, m.evmKeeper)
}
