package websocket

//From github.com/recws-org/recws

import (
	"crypto/tls"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
)

// ErrNotConnected is returned when the application read/writes
// a message and the connection is closed
var ErrNotConnected = errors.New("websocket: not connected")

// The RecConnClientWS type represents a Reconnecting WebSocket connection.
type RecConnClientWS struct {
	// RecIntvlMin specifies the initial reconnecting interval,
	// default to 2 seconds
	RecIntvlMin time.Duration
	// RecIntvlMax specifies the maximum reconnecting interval,
	// default to 30 seconds
	RecIntvlMax time.Duration
	// RecIntvlFactor specifies the rate of increase of the reconnection
	// interval, default to 1.5
	RecIntvlFactor float64
	// HandshakeTimeout specifies the duration for the handshake to complete,
	// default to 2 seconds
	HandshakeTimeout time.Duration
	// Proxy specifies the proxy function for the dialer
	// defaults to ProxyFromEnvironment
	Proxy func(*http.Request) (*url.URL, error)
	// Client TLS config to use on reconnect
	TLSClientConfig *tls.Config
	// SubscribeHandler fires after the connection successfully establish.
	SubscribeHandler func() error
	// KeepAliveTimeout is an interval for sending ping/pong messages
	// disabled if 0
	KeepAliveTimeout time.Duration
	// NonVerbose suppress connecting/reconnecting messages.
	NonVerbose bool

	isConnected bool
	mu          sync.RWMutex
	url         string
	reqHeader   http.Header
	httpResp    *http.Response
	dialErr     error
	dialer      *websocket.Dialer
	Conn        *websocket.Conn
	Logger      *log.Logger
}

// CloseAndReconnect will try to reconnect.
func (rc *RecConnClientWS) CloseAndReconnect() {
	rc.Close()
	//go rc.connect()
}

// setIsConnected sets state for isConnected
func (rc *RecConnClientWS) setIsConnected(state bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.isConnected = state
}

func (rc *RecConnClientWS) getConn() *websocket.Conn {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.Conn
}

// Close closes the underlying network connection without
// sending or waiting for a close frame.
func (rc *RecConnClientWS) Close() {
	if rc.getConn() != nil {
		rc.mu.Lock()
		rc.Conn.Close()
		rc.mu.Unlock()
	}
	rc.setIsConnected(false)
}

// ReadMessage is a helper method for getting a reader
// using NextReader and reading from that reader to a buffer.
//
// If the connection is closed ErrNotConnected is returned
func (rc *RecConnClientWS) ReadMessage() (messageType int, message []byte, err error) {
	err = ErrNotConnected
	if !rc.IsConnected() {
		return messageType, message, errors.New("websocket disconnected when read message")
	}
	rc.mu.Lock()
	messageType, message, err = rc.Conn.ReadMessage()
	rc.mu.Unlock()
	return
}

// WriteMessage is a helper method for getting a writer using NextWriter,
// writing the message and closing the writer.
//
// If the connection is closed ErrNotConnected is returned
func (rc *RecConnClientWS) WriteMessage(messageType int, data []byte) error {
	err := ErrNotConnected
	if !rc.IsConnected() {
		return errors.New("websocket disconnected when write message")
	}
	rc.mu.Lock()
	err = rc.Conn.WriteMessage(messageType, data)
	rc.mu.Unlock()
	if err != nil {
		return err
	}
	return nil
}

func (rc *RecConnClientWS) setURL(url string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.url = url
}

func (rc *RecConnClientWS) setReqHeader(reqHeader http.Header) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.reqHeader = reqHeader
}

// parseURL parses current url
func (rc *RecConnClientWS) parseURL(urlStr string) (string, error) {
	if urlStr == "" {
		return "", errors.New("dial: url cannot be empty")
	}

	u, err := url.Parse(urlStr)

	if err != nil {
		return "", errors.New("url: " + err.Error())
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return "", errors.New("url: websocket uris must start with ws or wss scheme")
	}

	if u.User != nil {
		return "", errors.New("url: user name and password are not allowed in websocket URIs")
	}

	return urlStr, nil
}

func (rc *RecConnClientWS) setDefaultRecIntvlMin() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.RecIntvlMin == 0 {
		rc.RecIntvlMin = 2 * time.Second
	}
}

func (rc *RecConnClientWS) setDefaultRecIntvlMax() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.RecIntvlMax == 0 {
		rc.RecIntvlMax = 30 * time.Second
	}
}

func (rc *RecConnClientWS) setDefaultRecIntvlFactor() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.RecIntvlFactor == 0 {
		rc.RecIntvlFactor = 1.5
	}
}

func (rc *RecConnClientWS) setDefaultHandshakeTimeout() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.HandshakeTimeout == 0 {
		rc.HandshakeTimeout = 2 * time.Second
	}
}

func (rc *RecConnClientWS) setDefaultProxy() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Proxy == nil {
		rc.Proxy = http.ProxyFromEnvironment
	}
}

func (rc *RecConnClientWS) setDefaultDialer(tlsClientConfig *tls.Config, handshakeTimeout time.Duration) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.dialer = &websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
		Proxy:            rc.Proxy,
		TLSClientConfig:  tlsClientConfig,
	}
}

func (rc *RecConnClientWS) getHandshakeTimeout() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.HandshakeTimeout
}

func (rc *RecConnClientWS) getTLSClientConfig() *tls.Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.TLSClientConfig
}

func (rc *RecConnClientWS) SetTLSClientConfig(tlsClientConfig *tls.Config) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.TLSClientConfig = tlsClientConfig
}

// Dial creates a new client connection.
// The URL url specifies the host and request URI. Use requestHeader to specify
// the origin (Origin), subprotocols (Sec-WebSocket-Protocol) and cookies
// (Cookie). Use GetHTTPResponse() method for the response.Header to get
// the selected subprotocol (Sec-WebSocket-Protocol) and cookies (Set-Cookie).
func (rc *RecConnClientWS) Dial(urlStr string, reqHeader http.Header) {
	urlStr, err := rc.parseURL(urlStr)

	if err != nil {
		rc.Logger.Fatalf("Dial: %v", err)
	}

	// Config
	rc.setURL(urlStr)
	rc.setReqHeader(reqHeader)
	rc.setDefaultRecIntvlMin()
	rc.setDefaultRecIntvlMax()
	rc.setDefaultRecIntvlFactor()
	rc.setDefaultHandshakeTimeout()
	rc.setDefaultProxy()
	rc.setDefaultDialer(rc.getTLSClientConfig(), rc.getHandshakeTimeout())

	// Connect
	go rc.connect()

	// wait on first attempt
	time.Sleep(rc.getHandshakeTimeout())
}

// GetURL returns current connection url
func (rc *RecConnClientWS) GetURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.url
}

func (rc *RecConnClientWS) getNonVerbose() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.NonVerbose
}

func (rc *RecConnClientWS) getBackoff() *backoff.Backoff {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return &backoff.Backoff{
		Min:    rc.RecIntvlMin,
		Max:    rc.RecIntvlMax,
		Factor: rc.RecIntvlFactor,
		Jitter: true,
	}
}

func (rc *RecConnClientWS) hasSubscribeHandler() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.SubscribeHandler != nil
}

func (rc *RecConnClientWS) getKeepAliveTimeout() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.KeepAliveTimeout
}

func (rc *RecConnClientWS) writeControlPingMessage() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
}

func (rc *RecConnClientWS) keepAlive() {
	var (
		keepAliveResponse = new(keepAliveResponse)
		ticker            = time.NewTicker(rc.getKeepAliveTimeout())
	)

	rc.mu.Lock()
	rc.Conn.SetPongHandler(func(msg string) error {
		keepAliveResponse.setLastResponse()
		return nil
	})
	rc.mu.Unlock()

	go func() {
		defer ticker.Stop()

		for {
			if !rc.IsConnected() {
				continue
			}

			if err := rc.writeControlPingMessage(); err != nil {
				log.Println(err)
			}

			<-ticker.C
			if time.Since(keepAliveResponse.getLastResponse()) > rc.getKeepAliveTimeout() {
				rc.CloseAndReconnect()
				return
			}
		}
	}()
}

func (rc *RecConnClientWS) connect() {
	b := rc.getBackoff()
	rand.Seed(time.Now().UTC().UnixNano())

	for {
		nextItvl := b.Duration()
		wsConn, httpResp, err := rc.dialer.Dial(rc.url, rc.reqHeader)

		rc.mu.Lock()
		rc.Conn = wsConn
		rc.dialErr = err
		rc.isConnected = err == nil
		rc.httpResp = httpResp
		rc.mu.Unlock()

		if err == nil {
			if !rc.getNonVerbose() {
				log.Printf("%s 連線成功！\n", rc.url)

				if !rc.hasSubscribeHandler() {
					return
				}

				if err := rc.SubscribeHandler(); err != nil {
					rc.Logger.Fatalf("Dial: connect handler failed with %s", err.Error())
				}

				log.Printf("Dial: connect handler was successfully established with %s\n", rc.url)

				if rc.getKeepAliveTimeout() != 0 {
					rc.keepAlive()
				}
			}

			return
		}

		if !rc.getNonVerbose() {
			log.Println(err)
			log.Printf("%d 秒後重新連線...\n", nextItvl)
		}

		time.Sleep(nextItvl)
	}
}

// GetHTTPResponse returns the http response from the handshake.
// Useful when WebSocket handshake fails,
// so that callers can handle redirects, authentication, etc.
func (rc *RecConnClientWS) GetHTTPResponse() *http.Response {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.httpResp
}

// GetDialError returns the last dialer error.
// nil on successful connection.
func (rc *RecConnClientWS) GetDialError() error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.dialErr
}

// IsConnected returns the WebSocket connection state
func (rc *RecConnClientWS) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return rc.isConnected
}
