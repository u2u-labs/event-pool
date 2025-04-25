package ibft

import (
	"event-pool/ibft/messages/proto"
	"event-pool/network"
	"event-pool/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

type transport interface {
	Multicast(msg *proto.IBFTMessage) error
}

type gossipTransport struct {
	topic *network.Topic
}

func (g *gossipTransport) Multicast(msg *proto.IBFTMessage) error {
	return g.topic.Publish(msg)
}

func (i *backendIBFT) Multicast(msg *proto.IBFTMessage) {
	if err := i.transport.Multicast(msg); err != nil {
		i.logger.Error("fail to gossip", "err", err)
	}
}

// setupTransport sets up the gossip transport protocol
func (i *backendIBFT) setupTransport() error {
	// Define a new topic
	topic, err := i.network.NewTopic(ibftProto, &proto.IBFTMessage{})
	if err != nil {
		return err
	}

	// Subscribe to the newly created topic
	if err := topic.Subscribe(
		func(obj interface{}, _ peer.ID) {
			if !i.IsActiveValidator() {
				return
			}

			msg, ok := obj.(*proto.IBFTMessage)
			if !ok {
				i.logger.Error("invalid type assertion for message request")

				return
			}

			added := i.consensus.AddMessage(msg)

			i.logger.Debugw(
				"validator message received",
				"type", msg.Type.String(),
				"height", msg.GetView().Height,
				"round", msg.GetView().Round,
				"version", msg.GetView().Version,
				"addr", types.BytesToAddress(msg.From).String(),
				"isAdded", added,
			)
		},
	); err != nil {
		return err
	}

	i.transport = &gossipTransport{topic: topic}

	return nil
}
