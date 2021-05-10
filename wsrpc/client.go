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

	return &Client{wsconn}, nil
}

type Client struct {
	conn *websocket.Conn
}

func (c *Client) Send(message string) error {
	err := c.conn.WriteMessage(websocket.TextMessage, []byte("Ping"))
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Receive(ch chan string) {
	go func() {
		for {
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				// Should gracefully close the connection if we detect that the
				// connection is no longer active
				log.Println("read:", err)
				break
			}

			ch <- string(message)
		}
	}()
}

func (c *Client) CloseConn() {
	c.conn.Close()
}
