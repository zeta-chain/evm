package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/gogoproto/proto"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	// DefaultPriorityReduction is the default amount of price values required for 1 unit of priority.
	// Because priority is `int64` while price is `big.Int`, it's necessary to scale down the range to keep it more pratical.
	// The default value is the same as the `sdk.DefaultPowerReduction`.
	DefaultPriorityReduction = sdk.DefaultPowerReduction

	// EmptyCodeHash is keccak256 hash of nil to represent empty code.
	EmptyCodeHash = crypto.Keccak256(nil)
)

// IsEmptyCodeHash checks if the given byte slice represents an empty code hash.
func IsEmptyCodeHash(bz []byte) bool {
	return bytes.Equal(bz, EmptyCodeHash)
}

// DecodeTxResponse decodes a protobuf-encoded byte slice into TxResponse
func DecodeTxResponse(in []byte) (*MsgEthereumTxResponse, error) {
	responses, err := DecodeTxResponses(in)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return &MsgEthereumTxResponse{}, nil
	}
	return responses[0], nil
}

// DecodeTxResponses decodes a protobuf-encoded byte slice into TxResponses
func DecodeTxResponses(in []byte) ([]*MsgEthereumTxResponse, error) {
	if in == nil {
		return nil, nil
	}
	var txMsgData sdk.TxMsgData
	if err := proto.Unmarshal(in, &txMsgData); err != nil {
		return nil, err
	}
	responses := make([]*MsgEthereumTxResponse, 0, len(txMsgData.MsgResponses))
	for _, res := range txMsgData.MsgResponses {
		var response MsgEthereumTxResponse
		if res.TypeUrl != "/"+proto.MessageName(&response) {
			continue
		}
		err := proto.Unmarshal(res.Value, &response)
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to unmarshal tx response message data")
		}
		responses = append(responses, &response)
	}
	return responses, nil
}

func logsFromTxResponse(dst []*ethtypes.Log, rsp *MsgEthereumTxResponse, blockNumber uint64) []*ethtypes.Log {
	if dst == nil {
		dst = make([]*ethtypes.Log, 0, len(rsp.Logs))
	}

	txHash := common.HexToHash(rsp.Hash)
	for _, log := range rsp.Logs {
		// fill in the tx/block informations
		l := log.ToEthereum()
		l.TxHash = txHash
		l.BlockNumber = blockNumber
		dst = append(dst, l)
	}
	return dst
}

// TxLogsFromEvents parses ethereum logs from cosmos events for specific msg index
func TxLogsFromEvents(events []abci.Event, msgIndex int) ([]*ethtypes.Log, error) {
	index := msgIndex
	for _, event := range events {
		if event.Type != EventTypeTxLog {
			continue
		}

		if msgIndex > 0 {
			// not the eth tx we want
			msgIndex--
			continue
		}

		return ParseTxLogsFromEvent(event)
	}
	return nil, fmt.Errorf("eth tx logs not found for message index %d", index)
}

// ParseTxLogsFromEvent parse tx logs from one event
func ParseTxLogsFromEvent(event abci.Event) ([]*ethtypes.Log, error) {
	logs := make([]*Log, 0, len(event.Attributes))
	for _, attr := range event.Attributes {
		if attr.Key != AttributeKeyTxLog {
			continue
		}

		var log Log
		if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
			return nil, err
		}

		logs = append(logs, &log)
	}
	return LogsToEthereum(logs), nil
}

// DecodeTxLogsFromEvents decodes a protobuf-encoded byte slice into ethereum logs
func DecodeTxLogsFromEvents(in []byte, events []abci.Event, blockNumber uint64) ([]*ethtypes.Log, error) {
	txResponses, err := DecodeTxResponses(in)
	if err != nil {
		return nil, err
	}
	var logs []*ethtypes.Log
	for _, response := range txResponses {
		logs = logsFromTxResponse(logs, response, blockNumber)
	}
	if len(logs) == 0 {
		for _, event := range events {
			if event.Type != EventTypeTxLog {
				continue
			}
			txLogs, err := ParseTxLogsFromEvent(event)
			if err != nil {
				return nil, err
			}
			logs = append(logs, txLogs...)
		}
	}
	return logs, nil
}

// EncodeTransactionLogs encodes TransactionLogs slice into a protobuf-encoded byte slice.
func EncodeTransactionLogs(res *TransactionLogs) ([]byte, error) {
	return proto.Marshal(res)
}

// DecodeTransactionLogs decodes an protobuf-encoded byte slice into TransactionLogs
func DecodeTransactionLogs(data []byte) (TransactionLogs, error) {
	var logs TransactionLogs
	err := proto.Unmarshal(data, &logs)
	if err != nil {
		return TransactionLogs{}, err
	}
	return logs, nil
}

// UnwrapEthereumMsg extracts MsgEthereumTx from wrapping sdk.Tx
func UnwrapEthereumMsg(tx *sdk.Tx, ethHash common.Hash) (*MsgEthereumTx, error) {
	if tx == nil {
		return nil, fmt.Errorf("invalid tx: nil")
	}

	for _, msg := range (*tx).GetMsgs() {
		ethMsg, ok := msg.(*MsgEthereumTx)
		if !ok {
			return nil, fmt.Errorf("invalid tx type: %T", tx)
		}
		txHash := ethMsg.AsTransaction().Hash()
		if txHash == ethHash {
			return ethMsg, nil
		}
	}

	return nil, fmt.Errorf("eth tx not found: %s", ethHash)
}

// UnpackEthMsg unpacks an Ethereum message from a Cosmos SDK message
func UnpackEthMsg(msg sdk.Msg) (
	ethMsg *MsgEthereumTx,
	ethTx *ethtypes.Transaction,
	err error,
) {
	msgEthTx, ok := msg.(*MsgEthereumTx)
	if !ok {
		return nil, nil, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid message type %T, expected %T", msg, (*MsgEthereumTx)(nil))
	}

	// sender address should be in the tx cache from the previous AnteHandle call
	return msgEthTx, msgEthTx.Raw.Transaction, nil
}

// BinSearch executes the binary search and hone in on an executable gas limit
func BinSearch(lo, hi uint64, executable func(uint64) (bool, *MsgEthereumTxResponse, error)) (uint64, error) {
	for lo+1 < hi {
		mid := (hi + lo) / 2
		failed, _, err := executable(mid)
		// If the error is not nil(consensus error), it means the provided message
		// call or transaction will never be accepted no matter how much gas it is
		// assigned. Return the error directly, don't struggle any more.
		if err != nil {
			return 0, err
		}
		if failed {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi, nil
}

// EffectiveGasPrice computes the effective gas price based on eip-1559 rules
// `effectiveGasPrice = min(baseFee + tipCap, feeCap)`
func EffectiveGasPrice(baseFee, feeCap, tipCap *big.Int) *big.Int {
	calcVal := new(big.Int).Add(tipCap, baseFee)
	if calcVal.Cmp(feeCap) < 0 {
		return calcVal
	}
	return feeCap
}

// HexAddress encode ethereum address without checksum, faster to run for state machine
func HexAddress(a []byte) string {
	var buf [common.AddressLength*2 + 2]byte
	copy(buf[:2], "0x")
	hex.Encode(buf[2:], a)
	return string(buf[:])
}

// SortedKVStoreKeys returns a slice of *KVStoreKey sorted by their map key.
func SortedKVStoreKeys(keys map[string]*storetypes.KVStoreKey) []*storetypes.KVStoreKey {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)

	sorted := make([]*storetypes.KVStoreKey, 0, len(keys))
	for _, name := range names {
		sorted = append(sorted, keys[name])
	}
	return sorted
}
