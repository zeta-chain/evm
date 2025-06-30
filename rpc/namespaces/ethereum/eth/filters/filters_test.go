package filters

import (
	"context"
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/stretchr/testify/require"

	filtermocks "github.com/cosmos/evm/rpc/namespaces/ethereum/eth/filters/mocks"
	"github.com/cosmos/evm/rpc/types"

	"cosmossdk.io/log"
)

func TestFilter(t *testing.T) {
	logger := log.NewNopLogger()
	testCases := []struct {
		name         string
		filter       filters.FilterCriteria
		expectations func(b *filtermocks.Backend)
		expLogs      []*ethtypes.Log
		expErr       string
	}{
		{
			name:   "invalid block range returns error",
			filter: filters.FilterCriteria{FromBlock: big.NewInt(100), ToBlock: big.NewInt(110)},
			expectations: func(b *filtermocks.Backend) {
				b.EXPECT().HeaderByNumber(types.EthLatestBlockNumber).Return(&ethtypes.Header{Number: big.NewInt(5)}, nil)
			},
			expErr: "invalid block range params",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend := filtermocks.NewBackend(t)
			f := newFilter(logger, backend, tc.filter, nil)
			tc.expectations(backend)
			logs, err := f.Logs(context.Background(), 15, 50)
			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
			}

			if tc.expLogs != nil {
				require.Equal(t, tc.expLogs, logs)
			}
		})
	}
}
