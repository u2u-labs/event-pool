package network

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	// subscribeOutputBufferSize is the size of subscribe output buffer in go-libp2p-pubsub
	// we should have enough capacity of the queue
	// because when queue is full, if the consumer does not read fast enough, new messages are dropped
	subscribeOutputBufferSize = 1024
)

type Topic struct {
	logger *zap.SugaredLogger

	topic     *pubsub.Topic
	typ       reflect.Type
	closeCh   chan struct{}
	closed    *uint64
	waitGroup sync.WaitGroup
}

func (t *Topic) createObj() proto.Message {
	message, ok := reflect.New(t.typ).Interface().(proto.Message)
	if !ok {
		return nil
	}

	return message
}

func (t *Topic) Close() {
	if t.closed == nil {
		return
	}

	if atomic.SwapUint64(t.closed, 1) > 0 {
		// Already closed.
		return
	}

	close(t.closeCh)   // close all subscribers
	t.waitGroup.Wait() // wait for all the subscribers to finish

	// if all subscribers are finished, close the topic
	if t.topic != nil {
		t.topic.Close()
		t.topic = nil
	}
}

func (t *Topic) Publish(obj proto.Message) error {
	data, err := proto.Marshal(obj)
	if err != nil {
		return err
	}

	return t.topic.Publish(context.Background(), data)
}

func (t *Topic) Subscribe(handler func(obj interface{}, from peer.ID)) error {
	sub, err := t.topic.Subscribe(pubsub.WithBufferSize(subscribeOutputBufferSize))
	if err != nil {
		return err
	}

	go t.readLoop(sub, handler)

	return nil
}

func (t *Topic) readLoop(sub *pubsub.Subscription, handler func(obj interface{}, from peer.ID)) {

	t.waitGroup.Add(1)
	defer t.waitGroup.Done()
	ctx, cancelFn := context.WithCancel(context.Background())

	go func() {
		<-t.closeCh
		cancelFn()
	}()

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			t.logger.Error("failed to get topic", "err", err)

			continue
		}

		go func() {
			obj := t.createObj()
			if err := proto.Unmarshal(msg.Data, obj); err != nil {
				t.logger.Error("failed to unmarshal topic", "err", err)

				return
			}

			handler(obj, msg.GetFrom())
		}()
	}
}

func (s *Server) NewTopic(protoID string, obj proto.Message) (*Topic, error) {
	topic, err := s.ps.Join(protoID)
	if err != nil {
		return nil, err
	}

	tt := &Topic{
		logger: s.logger.Named(protoID),
		topic:  topic,
		typ:    reflect.TypeOf(obj).Elem(),
	}

	return tt, nil
}
