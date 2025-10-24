package suite

import (
	"fmt"
	"maps"
	"math/big"
	"slices"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
)

// NonceAt returns the account nonce for the given account at the latest block
func (s *SystemTestSuite) NonceAt(nodeID string, accID string) (uint64, error) {
	ctx, cli, addr := s.EthClient.Setup(nodeID, accID)
	blockNumber, err := s.EthClient.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return uint64(0), fmt.Errorf("failed to get block number from %s: %v", nodeID, err)
	}
	if int64(blockNumber) < 0 {
		return uint64(0), fmt.Errorf("invaid block number %d", blockNumber)
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

// GetLatestBaseFee returns the base fee of the latest block
func (s *SystemTestSuite) GetLatestBaseFee(nodeID string) (*big.Int, error) {
	ctx, cli, _ := s.EthClient.Setup(nodeID, "acc0")
	blockNumber, err := cli.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block number from %s: %v", nodeID, err)
	}
	if int64(blockNumber) < 0 {
		return nil, fmt.Errorf("invaid block number %d", blockNumber)
	}

	block, err := cli.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block from %s: %v", nodeID, err)
	}

	if block.BaseFee().Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("failed to get block from %s: %v", nodeID, err)
	}

	return block.BaseFee(), nil
}

// BaseFee returns the base fee of the latest block
func (s *SystemTestSuite) WaitForCommit(
	nodeID string,
	txHash string,
	txType string,
	timeout time.Duration,
) error {
	switch txType {
	case TxTypeEVM:
		return s.waitForEthCommmit(nodeID, txHash, timeout)
	case TxTypeCosmos:
		return s.waitForCosmosCommmit(nodeID, txHash, timeout)
	default:
		return fmt.Errorf("invalid txtype: %s", txType)
	}
}

// waitForEthCommmit waits for the given eth tx to be committed within the timeout duration
func (s *SystemTestSuite) waitForEthCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	receipt, err := s.EthClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if receipt.Status != 1 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", txHash, err)
	}

	return nil
}

// waitForCosmosCommmit waits for the given cosmos tx to be committed within the timeout duration
func (s *SystemTestSuite) waitForCosmosCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	result, err := s.CosmosClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if result.TxResult.Code != 0 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", result.Hash.String(), err)
	}

	return nil
}

// CheckTxsPending checks if the given tx is either pending or committed within the timeout duration
func (s *SystemTestSuite) CheckTxPending(
	nodeID string,
	txHash string,
	txType string,
	timeout time.Duration,
) error {
	switch txType {
	case TxTypeEVM:
		return s.EthClient.CheckTxsPending(nodeID, txHash, timeout)
	case TxTypeCosmos:
		return s.CosmosClient.CheckTxsPending(nodeID, txHash, timeout)
	default:
		return fmt.Errorf("invalid tx type")
	}
}

// TxPoolContent returns the pending and queued tx hashes in the tx pool of the given node
func (s *SystemTestSuite) TxPoolContent(nodeID string, txType string) (pendingTxs, queuedTxs []string, err error) {
	switch txType {
	case TxTypeEVM:
		return s.ethTxPoolContent(nodeID)
	case TxTypeCosmos:
		return s.cosmosTxPoolContent(nodeID)
	default:
		return nil, nil, fmt.Errorf("invalid tx type")
	}
}

// ethTxPoolContent returns the pending and queued tx hashes in the tx pool of the given node
func (s *SystemTestSuite) ethTxPoolContent(nodeID string) (pendingTxHashes, queuedTxHashes []string, err error) {
	pendingTxs, queuedTxs, err := s.EthClient.TxPoolContent(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get txpool content from eth client: %v", err)
	}

	return s.extractTxHashesSorted(pendingTxs), s.extractTxHashesSorted(queuedTxs), nil
}

// cosmosTxPoolContent returns the pending tx hashes in the tx pool of the given node
func (s *SystemTestSuite) cosmosTxPoolContent(nodeID string) (pendingTxHashes, queuedTxHashes []string, err error) {
	result, err := s.CosmosClient.UnconfirmedTxs(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call unconfired transactions from cosmos client: %v", err)
	}

	pendingtxHashes := make([]string, 0)
	for _, tx := range result.Txs {
		pendingtxHashes = append(pendingtxHashes, string(tx.Hash()))
	}

	return pendingtxHashes, nil, nil
}

// extractTxHashesSorted processes transaction maps in a deterministic order and returns flat slice of tx hashes
func (s *SystemTestSuite) extractTxHashesSorted(txMap map[string]map[string]*clients.EthRPCTransaction) []string {
	var result []string

	// Get addresses and sort them for deterministic iteration
	addresses := slices.Collect(maps.Keys(txMap))
	slices.Sort(addresses)

	// Process addresses in sorted order
	for _, addr := range addresses {
		txs := txMap[addr]

		// Sort transactions by nonce for deterministic ordering
		nonces := slices.Collect(maps.Keys(txs))
		slices.Sort(nonces)

		// Add transaction hashes to flat result slice
		for _, nonce := range nonces {
			result = append(result, txs[nonce].Hash.Hex())
		}
	}

	return result
}
