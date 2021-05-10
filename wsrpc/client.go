package wsrpc

import (
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type ClientConn struct {
	// ctx    context.Context
	// cancel context.CancelFunc

	target string

	dopts dialOptions
}

// dialOptions configure a Dial call. dialOptions are set by the DialOption
// values passed to Dial.
type dialOptions struct {
	privKey      ed25519.PrivateKey
	serverPubKey [ed25519.PublicKeySize]byte
}

// DialOption configures how we set up the connection.
type DialOption interface {
	apply(*dialOptions)
}

// funcDialOption wraps a function that modifies dialOptions into an
// implementation of the DialOption interface.
type funcDialOption struct {
	f func(*dialOptions)
}

func (fdo *funcDialOption) apply(do *dialOptions) {
	fdo.f(do)
}

func newFuncDialOption(f func(*dialOptions)) *funcDialOption {
	return &funcDialOption{
		f: f,
	}
}

// WithTransportCredentials returns a DialOption which configures a connection
// level security credentials (e.g., TLS/SSL).
func WithTransportCreds(privKey ed25519.PrivateKey, serverPubKey [ed25519.PublicKeySize]byte) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		fmt.Println("PubKey: ", serverPubKey)
		o.privKey = privKey
		o.serverPubKey = serverPubKey
	})
}

func defaultDialOptions() dialOptions {
	return dialOptions{
		// copts: transport.ConnectOptions{
		// 	WriteBufferSize: defaultWriteBufSize,
		// 	ReadBufferSize:  defaultReadBufSize,
		// 	UseProxy:        true,
		// },
	}
}

func Dial(target string, opts ...DialOption) (*Client, error) {
	cc := &ClientConn{
		target: target,
		dopts:  defaultDialOptions(),
	}

	for _, opt := range opts {
		opt.apply(&cc.dopts)
	}

	cert := newMinimalX509CertFromPrivateKey(cc.dopts.privKey)
	config := tls.Config{
		Certificates: []tls.Certificate{cert},

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		// We pin the self-signed server public key rn.
		// If we wanted to use a proper CA for the server public key,
		// InsecureSkipVerify and VerifyPeerCertificate should be
		// removed. (See also discussion in README.md)
		InsecureSkipVerify: true,
		VerifyPeerCertificate: verifyCertMatchesIdentity(map[[ed25519.PublicKeySize]byte]string{
			cc.dopts.serverPubKey: "server",
		}),
	}

	//
	d := websocket.Dialer{
		TLSClientConfig:  &config,
		HandshakeTimeout: 45 * time.Second,
	}

	wsconn, resp, err := d.Dial("wss://"+target, http.Header{})
	if err != nil {
		log.Fatal(err, resp)
	}

	return NewClient(wsconn), nil
}

type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	receive chan []byte
}

func NewClient(conn *websocket.Conn) *Client {
	c := &Client{
		conn:    conn,
		send:    make(chan []byte),
		receive: make(chan []byte),
	}

	go c.writePump()
	go c.readPump()

	return c
}

// TODO - Figure out a way to return errors
func (c *Client) Send(message string) error {
	c.send <- []byte(message)

	return nil
}

// writePump pumps messages from the client to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// server ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	// ticker := time.NewTicker(pingPeriod)
	defer func() {
		// ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Some error ocurred writing: %v", err)
				return
			}
			// case <-ticker.C:
			// 	conn.SetWriteDeadline(time.Now().Add(writeWait))
			// 	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			// 		return
			// 	}
		}
	}
}

func (c *Client) Receive(handler func(message []byte)) {
	for {
		msg := <-c.receive

		handler(msg)

		// _, message, err := c.conn.ReadMessage()
		// if err != nil {
		// 	// Should gracefully close the connection if we detect that the
		// 	// connection is no longer active
		// 	log.Println("read:", err)
		// 	break
		// }

		// ch <- string(message)
	}
}

// readPump pumps messages from the websocket connection.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.conn.Close()
	}()
	// c.conn.SetReadLimit(maxMessageSize)
	// c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		c.receive <- message
	}
}

func (c *Client) CloseConn() {
	c.conn.Close()
}
