package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	v7 "github.com/cosmos/evm/x/vm/migrations/v7"

	v6 "github.com/cosmos/evm/x/vm/migrations/v6"
	"github.com/cosmos/evm/x/vm/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	evmKeeper *Keeper
	akKeeper  types.AccountKeeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper *Keeper, ak types.AccountKeeper) Migrator {
	return Migrator{
		evmKeeper: keeper,
		akKeeper:  ak,
	}
}

// Migrate5to6 migrates the store from consensus version 5 to 6
func (m Migrator) Migrate5to6(ctx sdk.Context) error {
	return v6.MigrateStore(ctx, m.evmKeeper, m.akKeeper)
}

// Migrate6to7 migrates the store from consensus version 5 to 6
func (m Migrator) Migrate6to7(ctx sdk.Context) error {
	return v7.MigrateStore(ctx, m.evmKeeper, m.akKeeper)
}
