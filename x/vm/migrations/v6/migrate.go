package v6

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	legacytypes "github.com/cosmos/evm/legacy/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

type evmKeeper interface {
	SetCodeHash(ctx sdk.Context, addrBytes, hashBytes []byte)
	GetParams(ctx sdk.Context) (params evmtypes.Params)
	SetParams(ctx sdk.Context, params evmtypes.Params) error
}

func MigrateStore(ctx sdk.Context, ek evmKeeper, ak evmtypes.AccountKeeper) error {
	newParams := evmtypes.DefaultParams()
	newParams.EvmDenom = "azeta"
	newParams.AllowUnprotectedTxs = true // currently set to true on live networks

	err := ek.SetParams(ctx, newParams)
	if err != nil {
		return err
	}

	ak.IterateAccounts(ctx, func(account sdk.AccountI) (stop bool) {
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

	return nil
}
