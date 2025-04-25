package messages

import (
	"bytes"

	"event-pool/ibft/messages/proto"
)

type CommittedSeal struct {
	Signer    []byte
	Signature []byte
}

// ExtractCommittedSeals extracts the committed seals from the passed in messages
func ExtractCommittedSeals(commitMessages []*proto.IBFTMessage) []*CommittedSeal {
	committedSeals := make([]*CommittedSeal, 0)

	for _, commitMessage := range commitMessages {
		if commitMessage.Type != proto.MessageType_COMMIT {
			continue
		}

		committedSeals = append(committedSeals, ExtractCommittedSeal(commitMessage))
	}

	return committedSeals
}

// ExtractCommittedSeal extracts the committed seal from the passed in message
func ExtractCommittedSeal(commitMessage *proto.IBFTMessage) *CommittedSeal {
	commitData, _ := commitMessage.Payload.(*proto.IBFTMessage_CommitData)

	return &CommittedSeal{
		Signer:    commitMessage.From,
		Signature: commitData.CommitData.CommittedSeal,
	}
}

// ExtractCommitHash extracts the commit proposal hash from the passed in message
func ExtractCommitHash(commitMessage *proto.IBFTMessage) []byte {
	if commitMessage.Type != proto.MessageType_COMMIT {
		return nil
	}

	commitData, _ := commitMessage.Payload.(*proto.IBFTMessage_CommitData)

	return commitData.CommitData.ProposalHash
}

// ExtractProposal extracts the proposal from the passed in message
func ExtractProposal(proposalMessage *proto.IBFTMessage) []byte {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.IBFTMessage_PreprepareData)

	return preprepareData.PreprepareData.Proposal
}

// ExtractProposalHash extracts the proposal hash from the passed in message
func ExtractProposalHash(proposalMessage *proto.IBFTMessage) []byte {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.IBFTMessage_PreprepareData)

	return preprepareData.PreprepareData.ProposalHash
}

// ExtractRoundChangeCertificate extracts the RCC from the passed in message
func ExtractRoundChangeCertificate(proposalMessage *proto.IBFTMessage) *proto.RoundChangeCertificate {
	if proposalMessage.Type != proto.MessageType_PREPREPARE {
		return nil
	}

	preprepareData, _ := proposalMessage.Payload.(*proto.IBFTMessage_PreprepareData)

	return preprepareData.PreprepareData.Certificate
}

// ExtractPrepareHash extracts the prepare proposal hash from the passed in message
func ExtractPrepareHash(prepareMessage *proto.IBFTMessage) []byte {
	if prepareMessage.Type != proto.MessageType_PREPARE {
		return nil
	}

	prepareData, _ := prepareMessage.Payload.(*proto.IBFTMessage_PrepareData)

	return prepareData.PrepareData.ProposalHash
}

// ExtractLatestPC extracts the latest PC from the passed in message
func ExtractLatestPC(roundChangeMessage *proto.IBFTMessage) *proto.PreparedCertificate {
	if roundChangeMessage.Type != proto.MessageType_ROUND_CHANGE {
		return nil
	}

	rcData, _ := roundChangeMessage.Payload.(*proto.IBFTMessage_RoundChangeData)

	return rcData.RoundChangeData.LatestPreparedCertificate
}

// ExtractLastPreparedProposedBlock extracts the latest prepared proposed block from the passed in message
func ExtractLastPreparedProposedBlock(roundChangeMessage *proto.IBFTMessage) []byte {
	if roundChangeMessage.Type != proto.MessageType_ROUND_CHANGE {
		return nil
	}

	rcData, _ := roundChangeMessage.Payload.(*proto.IBFTMessage_RoundChangeData)

	return rcData.RoundChangeData.LastPreparedProposedBlock
}

// HasUniqueSenders checks if the messages have unique senders
func HasUniqueSenders(messages []*proto.IBFTMessage) bool {
	if len(messages) < 1 {
		return false
	}

	senderMap := make(map[string]struct{})

	for _, message := range messages {
		key := string(message.From)
		if _, exists := senderMap[key]; exists {
			return false
		}

		senderMap[key] = struct{}{}
	}

	return true
}

// HaveSameProposalHash checks if the messages have the same proposal hash
func HaveSameProposalHash(messages []*proto.IBFTMessage) bool {
	if len(messages) < 1 {
		return false
	}

	var hash []byte = nil

	for _, message := range messages {
		var extractedHash []byte

		switch message.Type {
		case proto.MessageType_PREPREPARE:
			extractedHash = ExtractProposalHash(message)
		case proto.MessageType_PREPARE:
			extractedHash = ExtractPrepareHash(message)
		default:
			return false
		}

		if hash == nil {
			// No previous hash for comparison,
			// set the first one as the reference, as
			// all of them need to be the same anyway
			hash = extractedHash
		}

		if !bytes.Equal(hash, extractedHash) {
			return false
		}
	}

	return true
}

// AllHaveLowerRound checks if all messages have the same round
func AllHaveLowerRound(messages []*proto.IBFTMessage, round uint64) bool {
	if len(messages) < 1 {
		return false
	}

	for _, message := range messages {
		if message.View.Round >= round {
			return false
		}
	}

	return true
}

// AllHaveSameHeight checks if all messages have the same height
func AllHaveSameHeight(messages []*proto.IBFTMessage, height uint64) bool {
	if len(messages) < 1 {
		return false
	}

	for _, message := range messages {
		if message.View.Height != height {
			return false
		}
	}

	return true
}
