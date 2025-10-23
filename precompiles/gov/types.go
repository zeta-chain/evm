package gov

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"

	"cosmossdk.io/core/address"
	sdkerrors "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// EventVote defines the event data for the Vote transaction.
type EventVote struct {
	Voter      common.Address
	ProposalId uint64 //nolint:revive
	Option     uint8
}

// EventVoteWeighted defines the event data for the VoteWeighted transaction.
type EventVoteWeighted struct {
	Voter      common.Address
	ProposalId uint64 //nolint:revive
	Options    WeightedVoteOptions
}

// VotesInput defines the input for the Votes query.
type VotesInput struct {
	ProposalId uint64 //nolint:revive
	Pagination query.PageRequest
}

// VotesOutput defines the output for the Votes query.
type VotesOutput struct {
	Votes        []WeightedVote
	PageResponse query.PageResponse
}

// VoteOutput is the output response returned by the vote query method.
type VoteOutput struct {
	Vote WeightedVote
}

// WeightedVote defines a struct of vote for vote split.
type WeightedVote struct {
	ProposalId uint64 //nolint:revive
	Voter      common.Address
	Options    []WeightedVoteOption
	Metadata   string
}

// WeightedVoteOption defines a unit of vote for vote split.
type WeightedVoteOption struct {
	Option uint8
	Weight string
}

// WeightedVoteOptions defines a slice of WeightedVoteOption.
type WeightedVoteOptions []WeightedVoteOption

// DepositInput defines the input for the Deposit query.
type DepositInput struct {
	ProposalId uint64 //nolint:revive
	Depositor  common.Address
}

// DepositOutput defines the output for the Deposit query.
type DepositOutput struct {
	Deposit DepositData
}

// DepositsInput defines the input for the Deposits query.
type DepositsInput struct {
	ProposalId uint64 //nolint:revive
	Pagination query.PageRequest
}

// DepositsOutput defines the output for the Deposits query.
type DepositsOutput struct {
	Deposits     []DepositData      `abi:"deposits"`
	PageResponse query.PageResponse `abi:"pageResponse"`
}

// TallyResultOutput defines the output for the TallyResult query.
type TallyResultOutput struct {
	TallyResult TallyResultData
}

// DepositData represents information about a deposit on a proposal
type DepositData struct {
	ProposalId uint64         `abi:"proposalId"` //nolint:revive
	Depositor  common.Address `abi:"depositor"`
	Amount     []cmn.Coin     `abi:"amount"`
}

// TallyResultData represents the tally result of a proposal
type TallyResultData struct {
	Yes        string
	Abstain    string
	No         string
	NoWithVeto string
}

// NewMsgSubmitProposal constructs a MsgSubmitProposal.
// args: [proposerAddress, jsonBlob, []cmn.CoinInput deposit]
func NewMsgSubmitProposal(args []interface{}, cdc codec.Codec, addrCdc address.Codec) (*govv1.MsgSubmitProposal, common.Address, error) {
	emptyAddr := common.Address{}
	// -------------------------------------------------------------------------
	// 1. Argument sanity
	// -------------------------------------------------------------------------
	if len(args) != 3 {
		return nil, emptyAddr, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	proposer, ok := args[0].(common.Address)
	if !ok || proposer == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposer, args[0])
	}

	// 1-a  JSON blob
	jsonBlob, ok := args[1].([]byte)
	if !ok || len(jsonBlob) == 0 {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposalJSON, "jsonBlob arg")
	}

	// 1-b  Deposit
	coins, err := cmn.ToCoins(args[2])
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	// -------------------------------------------------------------------------
	// 2. Call helper that does JSON→Msg→Any conversion and submits the proposal
	// -------------------------------------------------------------------------
	amt, err := cmn.NewSdkCoinsFromCoins(coins)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	// 1. Decode the envelope
	var prop struct {
		Messages  []json.RawMessage `json:"messages"`
		Metadata  string            `json:"metadata"`
		Title     string            `json:"title"`
		Summary   string            `json:"summary"`
		Expedited bool              `json:"expedited"`
	}
	if err := json.Unmarshal(jsonBlob, &prop); err != nil {
		return nil, emptyAddr, sdkerrors.Wrap(err, "invalid proposal JSON")
	}

	// 2. Decode each message
	msgs := make([]sdk.Msg, len(prop.Messages))
	for i, m := range prop.Messages {
		var msg sdk.Msg
		if err := cdc.UnmarshalInterfaceJSON(m, &msg); err != nil {
			return nil, emptyAddr, sdkerrors.Wrapf(err, "message %d", i)
		}
		msgs[i] = msg
	}

	// 3. Pack into Any
	anys := make([]*codectypes.Any, len(msgs))
	for i, m := range msgs {
		anyVal, err := codectypes.NewAnyWithValue(m)
		if err != nil {
			return nil, common.Address{}, err
		}
		anys[i] = anyVal
	}

	// 4. Build & dispatch MsgSubmitProposal
	proposerAddr, err := addrCdc.BytesToString(proposer.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode proposer address: %w", err)
	}
	smsg := &govv1.MsgSubmitProposal{
		Messages:       anys,
		InitialDeposit: amt,
		Proposer:       proposerAddr,
		Metadata:       prop.Metadata,
		Title:          prop.Title,
		Summary:        prop.Summary,
		Expedited:      prop.Expedited,
	}

	return smsg, proposer, nil
}

// NewMsgDeposit constructs a MsgDeposit.
// args: [depositorAddress, proposalID, []cmn.CoinInput deposit]
func NewMsgDeposit(args []interface{}, addrCdc address.Codec) (*govv1.MsgDeposit, common.Address, error) {
	emptyAddr := common.Address{}
	if len(args) != 3 {
		return nil, emptyAddr, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 3, len(args))
	}

	depositor, ok := args[0].(common.Address)
	if !ok || depositor == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDepositor, args[0])
	}

	proposalID, ok := args[1].(uint64)
	if !ok {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposalID, args[1])
	}

	coins, err := cmn.ToCoins(args[2])
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	amt, err := cmn.NewSdkCoinsFromCoins(coins)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	depositorAddr, err := addrCdc.BytesToString(depositor.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode depositor address: %w", err)
	}
	msg := &govv1.MsgDeposit{
		ProposalId: proposalID,
		Amount:     amt,
		Depositor:  depositorAddr,
	}

	return msg, depositor, nil
}

// NewMsgCancelProposal constructs a MsgCancelProposal.
// args: [proposerAddress, proposalID]
func NewMsgCancelProposal(args []interface{}, addrCdc address.Codec) (*govv1.MsgCancelProposal, common.Address, error) {
	emptyAddr := common.Address{}
	if len(args) != 2 {
		return nil, emptyAddr, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	proposer, ok := args[0].(common.Address)
	if !ok || proposer == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposer, args[0])
	}

	proposalID, ok := args[1].(uint64)
	if !ok {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposalID, args[1])
	}

	proposerAddr, err := addrCdc.BytesToString(proposer.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode proposer address: %w", err)
	}
	return govv1.NewMsgCancelProposal(
		proposalID,
		proposerAddr,
	), proposer, nil
}

// NewMsgVote creates a new MsgVote instance.
func NewMsgVote(args []interface{}, addrCdc address.Codec) (*govv1.MsgVote, common.Address, error) {
	if len(args) != 4 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	voterAddress, ok := args[0].(common.Address)
	if !ok || voterAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidVoter, args[0])
	}

	proposalID, ok := args[1].(uint64)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidProposalID, args[1])
	}

	option, ok := args[2].(uint8)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidOption, args[2])
	}

	metadata, ok := args[3].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidMetadata, args[3])
	}

	voterAddr, err := addrCdc.BytesToString(voterAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode voter address: %w", err)
	}
	msg := &govv1.MsgVote{
		ProposalId: proposalID,
		Voter:      voterAddr,
		Option:     govv1.VoteOption(option),
		Metadata:   metadata,
	}

	return msg, voterAddress, nil
}

// NewMsgVoteWeighted creates a new MsgVoteWeighted instance.
func NewMsgVoteWeighted(method *abi.Method, args []interface{}, addrCdc address.Codec) (*govv1.MsgVoteWeighted, common.Address, WeightedVoteOptions, error) {
	if len(args) != 4 {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	voterAddress, ok := args[0].(common.Address)
	if !ok || voterAddress == (common.Address{}) {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf(ErrInvalidVoter, args[0])
	}

	proposalID, ok := args[1].(uint64)
	if !ok {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf(ErrInvalidProposalID, args[1])
	}

	// Unpack the input struct
	var options WeightedVoteOptions
	arguments := abi.Arguments{method.Inputs[2]}
	if err := arguments.Copy(&options, []interface{}{args[2]}); err != nil {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf("error while unpacking args to Options struct: %s", err)
	}

	weightedOptions := make([]*govv1.WeightedVoteOption, len(options))
	for i, option := range options {
		weightedOptions[i] = &govv1.WeightedVoteOption{
			Option: govv1.VoteOption(option.Option),
			Weight: option.Weight,
		}
	}

	metadata, ok := args[3].(string)
	if !ok {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf(ErrInvalidMetadata, args[3])
	}

	voterAddr, err := addrCdc.BytesToString(voterAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf("failed to decode voter address: %w", err)
	}
	msg := &govv1.MsgVoteWeighted{
		ProposalId: proposalID,
		Voter:      voterAddr,
		Options:    weightedOptions,
		Metadata:   metadata,
	}

	return msg, voterAddress, options, nil
}

// ParseVotesArgs parses the arguments for the Votes query.
func ParseVotesArgs(method *abi.Method, args []interface{}) (*govv1.QueryVotesRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	var input VotesInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to VotesInput: %s", err)
	}

	return &govv1.QueryVotesRequest{
		ProposalId: input.ProposalId,
		Pagination: &input.Pagination,
	}, nil
}

func (vo *VotesOutput) FromResponse(res *govv1.QueryVotesResponse) (*VotesOutput, error) {
	vo.Votes = make([]WeightedVote, len(res.Votes))
	for i, v := range res.Votes {
		hexAddr, err := utils.HexAddressFromBech32String(v.Voter)
		if err != nil {
			return nil, err
		}
		options := make([]WeightedVoteOption, len(v.Options))
		for j, opt := range v.Options {
			options[j] = WeightedVoteOption{
				Option: uint8(opt.Option), //nolint:gosec // G115
				Weight: opt.Weight,
			}
		}
		vo.Votes[i] = WeightedVote{
			ProposalId: v.ProposalId,
			Voter:      hexAddr,
			Options:    options,
			Metadata:   v.Metadata,
		}
	}
	if res.Pagination != nil {
		vo.PageResponse = query.PageResponse{
			NextKey: res.Pagination.NextKey,
			Total:   res.Pagination.Total,
		}
	}
	return vo, nil
}

// ParseVoteArgs parses the arguments for the Votes query.
func ParseVoteArgs(args []interface{}, addrCdc address.Codec) (*govv1.QueryVoteRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	proposalID, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidProposalID, args[0])
	}

	voter, ok := args[1].(common.Address)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidVoter, args[1])
	}

	voterAddr, err := addrCdc.BytesToString(voter.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode voter address: %w", err)
	}
	return &govv1.QueryVoteRequest{
		ProposalId: proposalID,
		Voter:      voterAddr,
	}, nil
}

func (vo *VoteOutput) FromResponse(res *govv1.QueryVoteResponse) (*VoteOutput, error) {
	hexVoter, err := utils.HexAddressFromBech32String(res.Vote.Voter)
	if err != nil {
		return nil, err
	}
	vo.Vote.Voter = hexVoter
	vo.Vote.Metadata = res.Vote.Metadata
	vo.Vote.ProposalId = res.Vote.ProposalId

	options := make([]WeightedVoteOption, len(res.Vote.Options))
	for j, opt := range res.Vote.Options {
		options[j] = WeightedVoteOption{
			Option: uint8(opt.Option), //nolint:gosec // G115
			Weight: opt.Weight,
		}
	}
	vo.Vote.Options = options
	return vo, nil
}

// ParseDepositArgs parses the arguments for the Deposit query.
func ParseDepositArgs(args []interface{}, addrCdc address.Codec) (*govv1.QueryDepositRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	proposalID, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidProposalID, args[0])
	}

	depositor, ok := args[1].(common.Address)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidDepositor, args[1])
	}

	depositorAddr, err := addrCdc.BytesToString(depositor.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode depositor address: %w", err)
	}
	return &govv1.QueryDepositRequest{
		ProposalId: proposalID,
		Depositor:  depositorAddr,
	}, nil
}

// ParseDepositsArgs parses the arguments for the Deposits query.
func ParseDepositsArgs(method *abi.Method, args []interface{}) (*govv1.QueryDepositsRequest, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	var input DepositsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to DepositsInput: %s", err)
	}

	return &govv1.QueryDepositsRequest{
		ProposalId: input.ProposalId,
		Pagination: &input.Pagination,
	}, nil
}

// ParseTallyResultArgs parses the arguments for the TallyResult query.
func ParseTallyResultArgs(args []interface{}) (*govv1.QueryTallyResultRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	proposalID, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidProposalID, args[0])
	}

	return &govv1.QueryTallyResultRequest{
		ProposalId: proposalID,
	}, nil
}

func (do *DepositOutput) FromResponse(res *govv1.QueryDepositResponse) (*DepositOutput, error) {
	hexDepositor, err := utils.HexAddressFromBech32String(res.Deposit.Depositor)
	if err != nil {
		return nil, err
	}
	coins := make([]cmn.Coin, len(res.Deposit.Amount))
	for i, c := range res.Deposit.Amount {
		coins[i] = cmn.Coin{
			Denom:  c.Denom,
			Amount: c.Amount.BigInt(),
		}
	}
	do.Deposit = DepositData{
		ProposalId: res.Deposit.ProposalId,
		Depositor:  hexDepositor,
		Amount:     coins,
	}
	return do, nil
}

func (do *DepositsOutput) FromResponse(res *govv1.QueryDepositsResponse) (*DepositsOutput, error) {
	do.Deposits = make([]DepositData, len(res.Deposits))
	for i, d := range res.Deposits {
		hexDepositor, err := utils.HexAddressFromBech32String(d.Depositor)
		if err != nil {
			return nil, err
		}
		coins := make([]cmn.Coin, len(d.Amount))
		for j, c := range d.Amount {
			coins[j] = cmn.Coin{
				Denom:  c.Denom,
				Amount: c.Amount.BigInt(),
			}
		}
		do.Deposits[i] = DepositData{
			ProposalId: d.ProposalId,
			Depositor:  hexDepositor,
			Amount:     coins,
		}
	}
	if res.Pagination != nil {
		do.PageResponse = query.PageResponse{
			NextKey: res.Pagination.NextKey,
			Total:   res.Pagination.Total,
		}
	}
	return do, nil
}

func (tro *TallyResultOutput) FromResponse(res *govv1.QueryTallyResultResponse) *TallyResultOutput {
	tro.TallyResult = TallyResultData{
		Yes:        res.Tally.YesCount,
		Abstain:    res.Tally.AbstainCount,
		No:         res.Tally.NoCount,
		NoWithVeto: res.Tally.NoWithVetoCount,
	}
	return tro
}

// ProposalOutput defines the output for the Proposal query
type ProposalOutput struct {
	Proposal ProposalData
}

// ProposalsInput defines the input for the Proposals query
type ProposalsInput struct {
	ProposalStatus uint32
	Voter          common.Address
	Depositor      common.Address
	Pagination     query.PageRequest
}

// ProposalsOutput defines the output for the Proposals query
type ProposalsOutput struct {
	Proposals    []ProposalData
	PageResponse query.PageResponse
}

// ProposalData represents a governance proposal
type ProposalData struct {
	Id               uint64          `abi:"id"` //nolint
	Messages         []string        `abi:"messages"`
	Status           uint32          `abi:"status"`
	FinalTallyResult TallyResultData `abi:"finalTallyResult"`
	SubmitTime       uint64          `abi:"submitTime"`
	DepositEndTime   uint64          `abi:"depositEndTime"`
	TotalDeposit     []cmn.Coin      `abi:"totalDeposit"`
	VotingStartTime  uint64          `abi:"votingStartTime"`
	VotingEndTime    uint64          `abi:"votingEndTime"`
	Metadata         string          `abi:"metadata"`
	Title            string          `abi:"title"`
	Summary          string          `abi:"summary"`
	Proposer         common.Address  `abi:"proposer"`
}

// ParseProposalArgs parses the arguments for the Proposal query
func ParseProposalArgs(args []interface{}) (*govv1.QueryProposalRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	proposalID, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidProposalID, args[0])
	}

	return &govv1.QueryProposalRequest{
		ProposalId: proposalID,
	}, nil
}

// ParseProposalsArgs parses the arguments for the Proposals query
func ParseProposalsArgs(method *abi.Method, args []interface{}, addrCdc address.Codec) (*govv1.QueryProposalsRequest, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	var input ProposalsInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to ProposalsInput: %s", err)
	}

	voter := ""
	if input.Voter != (common.Address{}) {
		var err error
		voter, err = addrCdc.BytesToString(input.Voter.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode voter address: %w", err)
		}
	}

	depositor := ""
	if input.Depositor != (common.Address{}) {
		var err error
		depositor, err = addrCdc.BytesToString(input.Depositor.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode depositor address: %w", err)
		}
	}

	return &govv1.QueryProposalsRequest{
		ProposalStatus: govv1.ProposalStatus(input.ProposalStatus), //nolint:gosec // G115
		Voter:          voter,
		Depositor:      depositor,
		Pagination:     &input.Pagination,
	}, nil
}

func (po *ProposalOutput) FromResponse(res *govv1.QueryProposalResponse) (*ProposalOutput, error) {
	msgs := make([]string, len(res.Proposal.Messages))
	for i, msg := range res.Proposal.Messages {
		msgs[i] = msg.TypeUrl
	}

	coins := make([]cmn.Coin, len(res.Proposal.TotalDeposit))
	for i, c := range res.Proposal.TotalDeposit {
		coins[i] = cmn.Coin{
			Denom:  c.Denom,
			Amount: c.Amount.BigInt(),
		}
	}

	proposer, err := utils.HexAddressFromBech32String(res.Proposal.Proposer)
	if err != nil {
		return nil, err
	}

	po.Proposal = ProposalData{
		Id:       res.Proposal.Id,
		Messages: msgs,
		Status:   uint32(res.Proposal.Status), //nolint:gosec // G115
		FinalTallyResult: TallyResultData{
			Yes:        res.Proposal.FinalTallyResult.YesCount,
			Abstain:    res.Proposal.FinalTallyResult.AbstainCount,
			No:         res.Proposal.FinalTallyResult.NoCount,
			NoWithVeto: res.Proposal.FinalTallyResult.NoWithVetoCount,
		},
		SubmitTime:     uint64(res.Proposal.SubmitTime.Unix()),     //nolint:gosec // G115
		DepositEndTime: uint64(res.Proposal.DepositEndTime.Unix()), //nolint:gosec // G115
		TotalDeposit:   coins,
		Metadata:       res.Proposal.Metadata,
		Title:          res.Proposal.Title,
		Summary:        res.Proposal.Summary,
		Proposer:       proposer,
	}
	// The following fields are nil when proposal is in deposit period
	if res.Proposal.VotingStartTime != nil {
		po.Proposal.VotingStartTime = uint64(res.Proposal.VotingStartTime.Unix()) //nolint:gosec // G115
	}
	if res.Proposal.VotingEndTime != nil {
		po.Proposal.VotingEndTime = uint64(res.Proposal.VotingEndTime.Unix()) //nolint:gosec // G115
	}
	return po, nil
}

func (po *ProposalsOutput) FromResponse(res *govv1.QueryProposalsResponse) (*ProposalsOutput, error) {
	po.Proposals = make([]ProposalData, len(res.Proposals))
	for i, p := range res.Proposals {
		msgs := make([]string, len(p.Messages))
		for j, msg := range p.Messages {
			msgs[j] = msg.TypeUrl
		}

		coins := make([]cmn.Coin, len(p.TotalDeposit))
		for j, c := range p.TotalDeposit {
			coins[j] = cmn.Coin{
				Denom:  c.Denom,
				Amount: c.Amount.BigInt(),
			}
		}

		proposer, err := utils.HexAddressFromBech32String(p.Proposer)
		if err != nil {
			return nil, err
		}

		proposalData := ProposalData{
			Id:       p.Id,
			Messages: msgs,
			Status:   uint32(p.Status), //nolint:gosec // G115
			FinalTallyResult: TallyResultData{
				Yes:        p.FinalTallyResult.YesCount,
				Abstain:    p.FinalTallyResult.AbstainCount,
				No:         p.FinalTallyResult.NoCount,
				NoWithVeto: p.FinalTallyResult.NoWithVetoCount,
			},
			SubmitTime:     uint64(p.SubmitTime.Unix()),     //nolint:gosec // G115
			DepositEndTime: uint64(p.DepositEndTime.Unix()), //nolint:gosec // G115
			TotalDeposit:   coins,
			Metadata:       p.Metadata,
			Title:          p.Title,
			Summary:        p.Summary,
			Proposer:       proposer,
		}

		// The following fields are nil when proposal is in deposit period
		if p.VotingStartTime != nil {
			proposalData.VotingStartTime = uint64(p.VotingStartTime.Unix()) //nolint:gosec // G115
		}
		if p.VotingEndTime != nil {
			proposalData.VotingEndTime = uint64(p.VotingEndTime.Unix()) //nolint:gosec // G115
		}

		po.Proposals[i] = proposalData
	}

	if res.Pagination != nil {
		po.PageResponse = query.PageResponse{
			NextKey: res.Pagination.NextKey,
			Total:   res.Pagination.Total,
		}
	}
	return po, nil
}

// ParamsOutput contains the output data for the governance parameters query
type ParamsOutput struct {
	VotingPeriod               int64      `abi:"votingPeriod"`
	MinDeposit                 []cmn.Coin `abi:"minDeposit"`
	MaxDepositPeriod           int64      `abi:"maxDepositPeriod"`
	Quorum                     string     `abi:"quorum"`
	Threshold                  string     `abi:"threshold"`
	VetoThreshold              string     `abi:"vetoThreshold"`
	MinInitialDepositRatio     string     `abi:"minInitialDepositRatio"`
	ProposalCancelRatio        string     `abi:"proposalCancelRatio"`
	ProposalCancelDest         string     `abi:"proposalCancelDest"`
	ExpeditedVotingPeriod      int64      `abi:"expeditedVotingPeriod"`
	ExpeditedThreshold         string     `abi:"expeditedThreshold"`
	ExpeditedMinDeposit        []cmn.Coin `abi:"expeditedMinDeposit"`
	BurnVoteQuorum             bool       `abi:"burnVoteQuorum"`
	BurnProposalDepositPrevote bool       `abi:"burnProposalDepositPrevote"`
	BurnVoteVeto               bool       `abi:"burnVoteVeto"`
	MinDepositRatio            string     `abi:"minDepositRatio"`
}

// FromResponse populates the ParamsOutput from a query response
func (o *ParamsOutput) FromResponse(res *govv1.QueryParamsResponse) *ParamsOutput {
	o.VotingPeriod = res.Params.VotingPeriod.Nanoseconds()
	o.MinDeposit = cmn.NewCoinsResponse(res.Params.MinDeposit)
	o.MaxDepositPeriod = res.Params.MaxDepositPeriod.Nanoseconds()
	o.Quorum = res.Params.Quorum
	o.Threshold = res.Params.Threshold
	o.VetoThreshold = res.Params.VetoThreshold
	o.MinInitialDepositRatio = res.Params.MinInitialDepositRatio
	o.ProposalCancelRatio = res.Params.ProposalCancelRatio
	o.ProposalCancelDest = res.Params.ProposalCancelDest
	o.ExpeditedVotingPeriod = res.Params.ExpeditedVotingPeriod.Nanoseconds()
	o.ExpeditedThreshold = res.Params.ExpeditedThreshold
	o.ExpeditedMinDeposit = cmn.NewCoinsResponse(res.Params.ExpeditedMinDeposit)
	o.BurnVoteQuorum = res.Params.BurnVoteQuorum
	o.BurnProposalDepositPrevote = res.Params.BurnProposalDepositPrevote
	o.BurnVoteVeto = res.Params.BurnVoteVeto
	o.MinDepositRatio = res.Params.MinDepositRatio
	return o
}

// BuildQueryParamsRequest returns the structure for the governance parameters query.
func BuildQueryParamsRequest(args []interface{}) (*govv1.QueryParamsRequest, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 0, len(args))
	}

	return &govv1.QueryParamsRequest{
		ParamsType: "",
	}, nil
}

// BuildQueryConstitutionRequest validates the args (none expected).
func BuildQueryConstitutionRequest(args []interface{}) (*govv1.QueryConstitutionRequest, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 0, len(args))
	}
	return &govv1.QueryConstitutionRequest{}, nil
}
