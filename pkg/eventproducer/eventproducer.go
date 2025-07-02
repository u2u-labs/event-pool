package eventproducer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"event-pool/types"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type EventProducer interface {
	Publish(evt types.EventRunnerPayload)
	Stop()
}

type RunnerEventProducer struct {
	rdb    *redis.Client
	logger *zap.SugaredLogger
	ch     chan types.EventRunnerPayload
}

func NewRunnerEventProducer(rdb *redis.Client, logger *zap.SugaredLogger) *RunnerEventProducer {
	return &RunnerEventProducer{
		rdb:    rdb,
		logger: logger,
		ch:     make(chan types.EventRunnerPayload, 100),
	}
}

func (p *RunnerEventProducer) Start() {
	go func() {
		for evt := range p.ch {
			p.handle(evt)
		}
	}()
}

func (p *RunnerEventProducer) handle(evt types.EventRunnerPayload) {
	channelKey := strings.ToLower(fmt.Sprintf("event_log_%d_%s_%s", evt.ChainID, evt.ContractAddress, evt.EventName))
	data, err := json.Marshal(evt)
	if err != nil {
		p.logger.Errorw("error marshaling event", "err", err)
		return
	}

	err = p.rdb.Publish(context.Background(), channelKey, data).Err()
	if err != nil {
		p.logger.Errorf("failed to publish event: %v", err)
	}
}

func (p *RunnerEventProducer) Publish(evt types.EventRunnerPayload) {
	select {
	case p.ch <- evt:
	}
	//blocking when full
}

func (p *RunnerEventProducer) Stop() {
	close(p.ch)
}
