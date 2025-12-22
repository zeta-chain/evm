package cosmos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseChainID(t *testing.T) {
	testCases := []struct {
		name        string
		chainID     string
		expected    uint64
		expectError bool
	}{
		{
			name:        "pure numeric chain ID",
			chainID:     "7000",
			expected:    7000,
			expectError: false,
		},
		{
			name:        "zetachain mainnet format",
			chainID:     "zetachain_7000-1",
			expected:    7000,
			expectError: false,
		},
		{
			name:        "zetachain testnet format",
			chainID:     "zetachain_7001-1",
			expected:    7001,
			expectError: false,
		},
		{
			name:        "chain with higher revision",
			chainID:     "mychain_12345-99",
			expected:    12345,
			expectError: false,
		},
		{
			name:        "chain ID with underscore in name - not supported",
			chainID:     "my_chain_100-1",
			expected:    0,
			expectError: true,
		},
		{
			name:        "missing underscore",
			chainID:     "zetachain7000-1",
			expected:    0,
			expectError: true,
		},
		{
			name:        "missing dash",
			chainID:     "zetachain_7000",
			expected:    0,
			expectError: true,
		},
		{
			name:        "underscore after dash",
			chainID:     "zetachain-7000_1",
			expected:    0,
			expectError: true,
		},
		{
			name:        "non-numeric EVM chain ID",
			chainID:     "zetachain_abc-1",
			expected:    0,
			expectError: true,
		},
		{
			name:        "empty string",
			chainID:     "",
			expected:    0,
			expectError: true,
		},
		{
			name:        "only underscore and dash",
			chainID:     "_-",
			expected:    0,
			expectError: true,
		},
		{
			name:        "large numeric chain ID",
			chainID:     "18446744073709551615",
			expected:    18446744073709551615,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseChainID(tc.chainID)

			if tc.expectError {
				require.Error(t, err, "expected error for chain ID: %s", tc.chainID)
			} else {
				require.NoError(t, err, "unexpected error for chain ID: %s", tc.chainID)
				require.Equal(t, tc.expected, result, "unexpected result for chain ID: %s", tc.chainID)
			}
		})
	}
}
