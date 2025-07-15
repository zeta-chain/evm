package v6

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

type evmKeeper interface {
	SetCodeHash(ctx sdk.Context, addrBytes, hashBytes []byte)
}

func MigrateStore(ctx sdk.Context, ek evmKeeper, ak evmtypes.AccountKeeper) error {
	fmt.Println("MigrateStore V5->V6")

	ak.IterateAccounts(ctx, func(account sdk.AccountI) (stop bool) {
		fmt.Println("Trying to migrate account", account.GetAddress().String())
		ethAcc, ok := account.(*types.EthAccount)
		if !ok {
			fmt.Println("Skip account")
			return false
		}

		fmt.Println("Migrating start")

		// NOTE: we only need to add store entries for smart contracts
		codeHashBytes := common.HexToHash(ethAcc.CodeHash).Bytes()
		if !evmtypes.IsEmptyCodeHash(codeHashBytes) {
			ethAddr := common.BytesToAddress(ethAcc.GetAddress().Bytes())
			ek.SetCodeHash(ctx, ethAddr.Bytes(), codeHashBytes)
		}

		// Set the base account in the account keeper instead of the EthAccount
		ak.SetAccount(ctx, ethAcc.BaseAccount)

		fmt.Println("Migrating account success")
		return false
	})

	fmt.Println("Migrating success")
	return nil
}
