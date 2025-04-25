package ibft

import (
	"google.golang.org/protobuf/proto"

	protoIBFT "event-pool/ibft/messages/proto"
	"event-pool/types"
)

func (i *backendIBFT) signMessage(msg *protoIBFT.IBFTMessage) *protoIBFT.IBFTMessage {
	raw, err := proto.Marshal(msg)
	if msg == nil {
		i.logger.Error("message pointed to nil")
	}

	if err != nil {
		i.logger.Error("failed to marshal message", "err", err)
		return nil
	}

	if msg.Signature, err = i.currentSigner.SignIBFTMessage(raw); err != nil {
		i.logger.Error("failed to sign ibft message", "err", err)
		return nil
	}

	return msg
}

func (i *backendIBFT) BuildPrePrepareMessage(
	proposal []byte,
	certificate *protoIBFT.RoundChangeCertificate,
	view *protoIBFT.View,
) *protoIBFT.IBFTMessage {
	block := &types.Block{}
	if err := block.UnmarshalRLP(proposal); err != nil {
		i.logger.Error("failed to unmarshal block proposal", "err", err)
		return nil
	}

	proposalHash := block.Hash().Bytes()

	msg := &protoIBFT.IBFTMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_PREPREPARE,
		Payload: &protoIBFT.IBFTMessage_PreprepareData{
			PreprepareData: &protoIBFT.PrePrepareMessage{
				Proposal:     proposal,
				ProposalHash: proposalHash,
				Certificate:  certificate,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildPrepareMessage(proposalHash []byte, view *protoIBFT.View) *protoIBFT.IBFTMessage {
	msg := &protoIBFT.IBFTMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_PREPARE,
		Payload: &protoIBFT.IBFTMessage_PrepareData{
			PrepareData: &protoIBFT.PrepareMessage{
				ProposalHash: proposalHash,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildCommitMessage(proposalHash []byte, view *protoIBFT.View) *protoIBFT.IBFTMessage {
	committedSeal, err := i.currentSigner.CreateCommittedSeal(proposalHash)
	if err != nil {
		i.logger.Error("Unable to build commit message, %v", err)

		return nil
	}

	msg := &protoIBFT.IBFTMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_COMMIT,
		Payload: &protoIBFT.IBFTMessage_CommitData{
			CommitData: &protoIBFT.CommitMessage{
				ProposalHash:  proposalHash,
				CommittedSeal: committedSeal,
			},
		},
	}

	return i.signMessage(msg)
}

func (i *backendIBFT) BuildRoundChangeMessage(
	proposal []byte,
	certificate *protoIBFT.PreparedCertificate,
	view *protoIBFT.View,
) *protoIBFT.IBFTMessage {
	msg := &protoIBFT.IBFTMessage{
		View: view,
		From: i.ID(),
		Type: protoIBFT.MessageType_ROUND_CHANGE,
		Payload: &protoIBFT.IBFTMessage_RoundChangeData{RoundChangeData: &protoIBFT.RoundChangeMessage{
			LastPreparedProposedBlock: proposal,
			LatestPreparedCertificate: certificate,
		}},
	}

	return i.signMessage(msg)

}
