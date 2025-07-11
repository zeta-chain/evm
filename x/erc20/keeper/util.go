package keeper

import (
	"cosmossdk.io/errors"
	types2 "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/types"
)

// monitorApprovalEvent returns an error if the given transactions logs include
// an unexpected `Approval` event
func (k Keeper) monitorApprovalEvent(res *types.MsgEthereumTxResponse) error {
	if res == nil || len(res.Logs) == 0 {
		return nil
	}

	for _, log := range res.Logs {
		if log.Topics[0] == logApprovalSigHash.Hex() {
			return errors.Wrapf(
				types2.ErrUnexpectedEvent, "unexpected Approval event",
			)
		}
	}

	return nil
}

// monitorApprovalEvent returns an error if the given transactions logs DO NOT include
// an expected `Transfer` event
func (k Keeper) monitorTransferEvent(res *types.MsgEthereumTxResponse) error {
	if res == nil || len(res.Logs) == 0 {
		return errors.Wrapf(
			types2.ErrExpectedEvent, "expected Transfer event",
		)
	}

	for _, log := range res.Logs {
		if log.Topics[0] == logTransferSigHash.Hex() {
			return nil
		}
	}

	return errors.Wrapf(
		types2.ErrExpectedEvent, "expected Transfer event",
	)
}
