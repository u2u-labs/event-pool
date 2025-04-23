package txpool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"event-pool/helper/hex"
	"event-pool/txpool/proto"
	"event-pool/types"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

// Status implements the GRPC status endpoint. Returns the number of transactions in the pool
func (p *TxPool) Status(ctx context.Context, req *empty.Empty) (*proto.TxnPoolStatusResp, error) {
	resp := &proto.TxnPoolStatusResp{
		Length: p.accounts.promoted(),
	}

	return resp, nil
}

// AddTxn adds a local transaction to the pool
func (p *TxPool) AddTxn(ctx context.Context, raw *proto.AddTxnReq) (*proto.AddTxnResp, error) {
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("transaction's field raw is empty")
	}

	txData, err := hex.DecodeHex(raw.Data)
	if err != nil {
		return nil, err
	}

	txn := new(types.Transaction)
	if err := txn.UnmarshalRLP(txData); err != nil {
		return nil, err
	}

	if raw.From != "" {
		from := types.Address{}
		if err := from.UnmarshalText([]byte(raw.From)); err != nil {
			return nil, err
		}

		txn.From = from
	}

	if err := p.AddTx(txn); err != nil {
		return nil, err
	}

	return &proto.AddTxnResp{
		TxHash: txn.Hash.String(),
	}, nil
}

// Subscribe implements the operator endpoint. It subscribes to new events in the tx pool
func (p *TxPool) Subscribe(
	request *proto.SubscribeRequest,
	stream proto.TxnPoolOperator_SubscribeServer,
) error {
	subscription := p.eventManager.subscribe(request.Types)

	cancel := func() {
		p.eventManager.cancelSubscription(subscription.subscriptionID)
	}

	for {
		select {
		case event, more := <-subscription.subscriptionChannel:
			if !more {
				// Subscription is closed from some other place
				return nil
			}

			if sendErr := stream.Send(event); sendErr != nil {
				cancel()

				return nil
			}
		case <-stream.Context().Done():
			cancel()

			return nil
		}
	}
}

// wsUpgrader defines upgrade parameters for the WS connection
var wsUpgrader = websocket.Upgrader{
	// Uses the default HTTP buffer sizes for Read / Write buffers.
	// Documentation specifies that they are 4096B in size.
	// There is no need to have them be 4x in size when requests / responses
	// shouldn't exceed 1024B
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (p *TxPool) HandleWs(w http.ResponseWriter, req *http.Request) {
	// CORS rule - Allow requests from anywhere
	wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// Upgrade the connection to a WS one
	ws, err := wsUpgrader.Upgrade(w, req, nil)
	if err != nil {
		p.logger.Error(fmt.Sprintf("Unable to upgrade to a WS connection, %s", err.Error()))

		return
	}
	subscription := p.eventManager2.subscribe([]proto.EventType{proto.EventType_SUBGRAPH_EVENT_ADDED})

	cancel := func() {
		p.eventManager2.cancelSubscription(subscription.subscriptionID)
	}

	// Defer WS closure
	defer func(ws *websocket.Conn) {
		cancel()
		err = ws.Close()
		if err != nil {
			p.logger.Error(
				fmt.Sprintf("Unable to gracefully close WS connection, %s", err.Error()),
			)
		}
	}(ws)

	wrapConn := &wsWrapper{ws: ws, logger: p.logger}

	p.logger.Info("Websocket connection established")
	// Run the listen loop

	for {
		// Read the incoming message
		msgType, _, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseAbnormalClosure,
			) {
				// Accepted close codes
				p.logger.Info("Closing WS connection gracefully")
			} else {
				p.logger.Error(fmt.Sprintf("Unable to read WS message, %s", err.Error()))
				p.logger.Info("Closing WS connection with error")
			}

			break
		}

		select {
		case event, more := <-subscription.subscriptionChannel:
			if !more {
				// Subscription is closed from some other place
				return
			}

			data := &proto.EventData{
				Payload: event.TxHash,
			}
			resp, err := json.Marshal(data)
			if err != nil {
				p.logger.Error(fmt.Sprintf("Unable to marshal WS message, %s", err.Error()))
				return
			}

			if sendErr := wrapConn.WriteMessage(msgType, resp); sendErr != nil {
				return
			}
		}
	}
}

// wsWrapper is a wrapping object for the web socket connection and logger
type wsWrapper struct {
	sync.Mutex

	ws       *websocket.Conn    // the actual WS connection
	logger   *zap.SugaredLogger // module logger
	filterID string             // filter ID
}

func (w *wsWrapper) SetFilterID(filterID string) {
	w.filterID = filterID
}

func (w *wsWrapper) GetFilterID() string {
	return w.filterID
}

// WriteMessage writes out the message to the WS peer
func (w *wsWrapper) WriteMessage(messageType int, data []byte) error {
	w.Lock()
	defer w.Unlock()
	writeErr := w.ws.WriteMessage(messageType, data)

	if writeErr != nil {
		w.logger.Error(
			fmt.Sprintf("Unable to write WS message, %s", writeErr.Error()),
		)
	}

	return writeErr
}

// isSupportedWSType returns a status indicating if the message type is supported
func isSupportedWSType(messageType int) bool {
	return messageType == websocket.TextMessage ||
		messageType == websocket.BinaryMessage
}
