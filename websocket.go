package websocket

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type SubscribeMessage struct {
	Op      string `json:"op"`
	Channel string `json:"channel,omitempty"`
	Market  string `json:"market,omitempty"`
}

type AuthenticationMessage struct {
	Args ArgsN  `json:"args"`
	Op   string `json:"op"`
}

type ArgsN struct {
	Key        string `json:"key"`
	Sign       string `json:"sign"`
	Time       int64  `json:"time"`
	Subaccount string `json:"subaccount"`
}

/*
func Connect (log *log.Logger) *RecConnClientWS {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial("wss://ftx.com/ws/", nil)
	if nil != err {
		log.Println(err)
	}
	return &ClientWS{
		Conn: conn,
		Logger:  log,
	}
}
*/

func (c *RecConnClientWS) ReadFromWSToChannel(ctx context.Context, chRead *chan []byte) {
	for {
		select {
		case <-ctx.Done():
			close(*chRead)
			return
		default:
			_, message, err := c.Conn.ReadMessage()
			if err != nil {
				close(*chRead)
				return
			}
			*chRead <- message
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
			c.Conn.WriteMessage(websocket.TextMessage, message.([]byte))
		default:
			time.Sleep(time.Second)
		}
	}
}

func (c *RecConnClientWS) GetAuthMessage(key string, secret string, subaccount string) []byte {
	nonce := makeTimestamp()
	req := fmt.Sprintf("%dwebsocket_login", nonce)
	sig := hmac.New(sha256.New, []byte(secret))
	sig.Write([]byte(req))
	signature := hex.EncodeToString(sig.Sum(nil))
	auth := AuthenticationMessage{Op: "login", Args: ArgsN{Key: key, Sign: signature, Time: nonce, Subaccount: subaccount}}
	message, err := json.Marshal(auth)
	if err != nil {
		c.Logger.Println(err)
	}
	return message
}

func (c *RecConnClientWS) GetSubscribeMessage(channel, market string) []byte {
	sub := SubscribeMessage{Op: "subscribe", Channel: channel, Market: market}
	message, err := json.Marshal(sub)
	if err != nil {
		c.Logger.Println(err)
	}
	return message
}

func (c *RecConnClientWS) GetPingPong() []byte {
	sub := SubscribeMessage{Op: "ping"}
	message, err := json.Marshal(sub)
	if err != nil {
		c.Logger.Println(err)
	}
	return message
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
