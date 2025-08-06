package v6

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	legacyevm "github.com/cosmos/evm/legacy/evm"
	legacytypes "github.com/cosmos/evm/legacy/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

type evmKeeper interface {
	SetCodeHash(ctx sdk.Context, addrBytes, hashBytes []byte)
	GetParams(ctx sdk.Context) (params evmtypes.Params)
	GetLegacyParams(ctx sdk.Context) (params legacyevm.Params)
	SetParams(ctx sdk.Context, params evmtypes.Params) error
}

func MigrateStore(ctx sdk.Context, ek evmKeeper, ak evmtypes.AccountKeeper) error {
	fmt.Println("MigrateStore V5->V6")

	fmt.Println("Migrating params")

	legacyParams := ek.GetLegacyParams(ctx)

	m, _ := legacyParams.Marshal()
	fmt.Println("Legacy params", legacyParams, string(m))

	newParams := evmtypes.DefaultParams()
	newParams.EvmDenom = legacyParams.EvmDenom
	newParams.ExtraEIPs = legacyParams.ExtraEIPs
	newParams.AllowUnprotectedTxs = legacyParams.AllowUnprotectedTxs

	m, _ = newParams.Marshal()
	fmt.Println("New params", newParams, string(m))

	err := ek.SetParams(ctx, newParams)
	if err != nil {
		fmt.Println("Set params error", err.Error())
		return err
	}

	fmt.Println("Migrating params success")

	fmt.Println("Migrating accounts")

	ak.IterateAccounts(ctx, func(account sdk.AccountI) (stop bool) {
		fmt.Println("Trying to migrate account", account.GetAddress().String())
		ethAcc, ok := account.(*legacytypes.EthAccount)
		if !ok {
			return false
		}

		// NOTE: we only need to add store entries for smart contracts
		codeHashBytes := common.HexToHash(ethAcc.CodeHash).Bytes()
		if !evmtypes.IsEmptyCodeHash(codeHashBytes) {
			ethAddr := common.BytesToAddress(ethAcc.GetAddress().Bytes())
			ek.SetCodeHash(ctx, ethAddr.Bytes(), codeHashBytes)
		}

		// Set the base account in the account keeper instead of the EthAccount
		ak.SetAccount(ctx, ethAcc.BaseAccount)

		return false
	})

	fmt.Println("Migrating success")
	return nil
}
