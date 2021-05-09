package wsrpc

// 1. Set up a HTTP server with HTTPs
// 2. Upgrade the connection to a websocket

import (
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// A ServerOption sets options such as credentials, codec and keepalive parameters, etc.
type ServerOption interface {
	apply(*serverOptions)
}

type serverOptions struct {
	privKey          ed25519.PrivateKey
	clientIdentities map[[ed25519.PublicKeySize]byte]string
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption struct {
	f func(*serverOptions)
}

func newFuncServerOption(f func(*serverOptions)) *funcServerOption {
	return &funcServerOption{
		f: f,
	}
}

func (fdo *funcServerOption) apply(do *serverOptions) {
	fdo.f(do)
}

// Creds returns a ServerOption that sets credentials for server connections.
func Creds(privKey ed25519.PrivateKey, clientIdentities map[[ed25519.PublicKeySize]byte]string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.privKey = privKey
		o.clientIdentities = clientIdentities
	})
}

type Server struct {
	opts serverOptions
}

var defaultServerOptions = serverOptions{
	// maxReceiveMessageSize: defaultServerMaxReceiveMessageSize,
	// maxSendMessageSize:    defaultServerMaxSendMessageSize,
	// connectionTimeout:     120 * time.Second,
	// writeBufferSize:       defaultWriteBufSize,
	// readBufferSize:        defaultReadBufSize,
}

func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	s := &Server{
		opts: opts,
	}

	return s
}

func (s *Server) Serve(lis net.Listener) {
	cert := newMinimalX509CertFromPrivateKey(s.opts.privKey)

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAnyClientCert,

		// Since our clients use self-signed certs, we skip verficiation here.
		// Instead, we use VerifyPeerCertificate for our own check
		InsecureSkipVerify: true,

		MaxVersion: tls.VersionTLS13,
		MinVersion: tls.VersionTLS13,

		VerifyPeerCertificate: verifyCertMatchesIdentity(s.opts.clientIdentities),
	}

	httpsrv := &http.Server{
		TLSConfig: &tlsConfig,
	}
	http.HandleFunc("/", wshandler)

	// The TLS config is store on the http.Server struct
	httpsrv.ServeTLS(lis, "", "")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 45 * time.Second,
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Establishing Websocket connection")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	go readWS(c)
	go writeWS(c)

	select {}
}

func readWS(c *websocket.Conn) {
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		// err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func writeWS(c *websocket.Conn) {
	for {
		err := c.WriteMessage(websocket.TextMessage, []byte("Pong"))
		if err != nil {
			log.Printf("Some error ocurred ponging: %v", err)
			return
		}

		log.Println("Sent: Pong")

		time.Sleep(5 * time.Second)
	}
}
