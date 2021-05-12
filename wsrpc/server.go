package wsrpc

// 1. Set up a HTTP server with HTTPs
// 2. Upgrade the connection to a websocket

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var ErrNotConnected = errors.New("client not connected")

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	//
	defaultWriteBufSize = 1024

	//
	defaultReadBufSize = 1024
)

// A ServerOption sets options such as credentials, codec and keepalive parameters, etc.
type ServerOption interface {
	apply(*serverOptions)
}

type serverOptions struct {
	// Buffer
	writeBufferSize int
	readBufferSize  int

	// Transport Credentials
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

// Buffer returns a ServerOption that sets buffer options
func Buffer(readSize int, writeSize int) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.readBufferSize = readSize
		o.writeBufferSize = writeSize
	})
}

var defaultServerOptions = serverOptions{
	writeBufferSize: defaultWriteBufSize,
	readBufferSize:  defaultReadBufSize,
}

type Server struct {
	opts serverOptions
	// Inbound messages from the clients.
	broadcast chan []byte
	// Holds a list of the open connections mapped to a buffered channel of
	// outbound messages.
	connections map[[ed25519.PublicKeySize]byte]chan []byte
	// Parameters for upgrading a websocket connection
	upgrader websocket.Upgrader
}

func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	s := &Server{
		opts: opts,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  opts.readBufferSize,
			WriteBufferSize: opts.writeBufferSize,
		},
		connections: map[[ed25519.PublicKeySize]byte]chan []byte{},
		broadcast:   make(chan []byte),
	}

	return s
}

func (s *Server) Serve(lis net.Listener) {
	tlsConfig := NewServerTLSConfig(s.opts.privKey, s.opts.clientIdentities)

	httpsrv := &http.Server{
		TLSConfig: &tlsConfig,
	}
	http.HandleFunc("/", s.wshandler)
	httpsrv.ServeTLS(lis, "", "")
}

func (s *Server) wshandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Establishing Websocket connection")

	// Ensure there is only a single connection per public key
	pk, err := pubKeyFromCert(r.TLS.PeerCertificates[0])
	if err != nil {
		log.Print("error: ", err)
		return
	}

	if _, ok := s.connections[pk]; ok {
		log.Println("error: You can only have one connection")

		return
	}

	c, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	// Set up a channel to send messages and register it to the connection
	// TODO - Protect with mutex?
	sendCh := make(chan []byte)
	s.connections[pk] = sendCh

	go s.readPump(c, pk)
	go s.writePump(c, pk, sendCh)

	select {}
}

func (s *Server) Send(pub [32]byte, message []byte) error {
	// Find the connection
	ch, ok := s.connections[pub]
	if !ok {
		return ErrNotConnected
	}

	// Send the message to the channel
	ch <- message

	return nil
}

func (s *Server) Broadcast() <-chan []byte {
	return s.broadcast
}

// readPump pumps messages from the websocket connection.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (s *Server) readPump(conn *websocket.Conn, pk [ed25519.PublicKeySize]byte) {
	defer func() {
		// c.hub.unregister <- c
		conn.Close()
	}()
	// c.conn.SetReadLimit(maxMessageSize)
	// c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// conn.SetPongHandler(func(string) error {
	// 	fmt.Println("Received Ping")
	// 	conn.SetReadDeadline(time.Now().Add(pongWait))
	// 	return nil
	// })

	conn.SetPingHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Remove the connection from the channel
				delete(s.connections, pk)

				log.Printf("error: %v", err)
			}
			break
		}

		s.broadcast <- message
	}
}

// writePump pumps messages from the server to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// server ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (s *Server) writePump(conn *websocket.Conn, pk [ed25519.PublicKeySize]byte, ch <-chan []byte) {
	// ticker := time.NewTicker(pingPeriod)
	defer func() {
		// ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-ch:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Closed the channel.
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Some error ocurred writing: %v", err)

				// Remove the connection from the channel
				delete(s.connections, pk)

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
