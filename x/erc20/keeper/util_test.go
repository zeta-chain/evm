package keeper

import (
	"github.com/cosmos/evm/x/vm/types"
	"testing"
	"github.com/stretchr/testify/require"
)

func TestMonitorApprovalEvent(t *testing.T) {
	k := Keeper{} // initialize as needed

	tests := []struct {
		name        string
		res         *types.MsgEthereumTxResponse
		expectError bool
	}{
		{
			name:        "nil response",
			res:         nil,
			expectError: false,
		},
		{
			name: "empty logs",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{},
			},
			expectError: false,
		},
		{
			name: "no approval event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "has approval event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{logApprovalSigHash.Hex()},
					},
				},
			},
			expectError: true,
		},
		{
			name: "approval event among others",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
					{
						Topics: []string{logApprovalSigHash.Hex()},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := k.monitorApprovalEvent(tt.res)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unexpected Approval event")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMonitorTransferEvent(t *testing.T) {
	k := Keeper{} // initialize as needed

	tests := []struct {
		name        string
		res         *types.MsgEthereumTxResponse
		expectError bool
	}{
		{
			name:        "nil response",
			res:         nil,
			expectError: true,
		},
		{
			name: "empty logs",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{},
			},
			expectError: true,
		},
		{
			name: "no transfer event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
				},
			},
			expectError: true,
		},
		{
			name: "has transfer event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{logTransferSigHash.Hex()},
					},
				},
			},
			expectError: false,
		},
		{
			name: "transfer event among others",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
					{
						Topics: []string{logTransferSigHash.Hex()},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := k.monitorTransferEvent(tt.res)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "expected Transfer event")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
