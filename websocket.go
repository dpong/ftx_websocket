package websocket

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func (c *RecConnClientWS) ReadFromWSToChannel(ctx context.Context, chRead *chan []byte, buffer int) {
	for {
		select {
		case <-ctx.Done():
			close(*chRead)
			return
		default:
			_, message, err := c.ReadMessage()
			if err != nil {
				close(*chRead)
				return
			}
			if len(*chRead) < buffer { // avoid full channel deadlock
				*chRead <- message
			}
		}
	}
}

func (c *RecConnClientWS) WriteFromChannelToWS(ctx context.Context, chWrite *chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-*chWrite:
			if !ok {
				return
			}
			err := c.WriteMessage(websocket.TextMessage, message.([]byte))
			if err != nil {
				return
			}
		default:
			time.Sleep(time.Second)
		}
	}
}
