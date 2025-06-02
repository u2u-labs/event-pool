package ws

import (
	"fmt"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"sync"
)

// WsWrapper is a wrapping object for the web socket connection and logger
type WsWrapper struct {
	sync.Mutex

	Ws       *websocket.Conn    // the actual WS connection
	Logger   *zap.SugaredLogger // module logger
	FilterID string             // filter ID
}

func (w *WsWrapper) SetFilterID(filterID string) {
	w.FilterID = filterID
}

func (w *WsWrapper) GetFilterID() string {
	return w.FilterID
}

// WriteMessage writes out the message to the WS peer
func (w *WsWrapper) WriteMessage(messageType int, data []byte) error {
	w.Lock()
	defer w.Unlock()
	writeErr := w.Ws.WriteMessage(messageType, data)

	if writeErr != nil {
		w.Logger.Error(
			fmt.Sprintf("Unable to write WS message, %s", writeErr.Error()),
		)
	}

	return writeErr
}
