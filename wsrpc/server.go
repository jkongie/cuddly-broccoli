package wsrpc

// 1. Set up a HTTP server with HTTPs
// 2. Upgrade the connection to a websocket

import (
	"crypto/ed25519"
	"crypto/tls"
	"log"
	"net"
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
	defer lis.Close()

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

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		tlsConn := tls.Server(conn, &tlsConfig)

		go s.acceptClient(tlsConn)
	}
}

func (s *Server) acceptClient(conn net.Conn) {
	defer conn.Close()

	// Cast to *tls.Conn so we have access to TLS specific functions
	tlsConn := conn.(*tls.Conn)

	// Perform handshake so that we know the public key
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("Closing connection, error during handshake: %v", err)
		return
	}

	// Get client name from public key
	pk, err := pubKeyFromCert(tlsConn.ConnectionState().PeerCertificates[0])
	if err != nil {
		log.Printf("Closing connection, error getting public key of client: %v", err)
		return
	}
	name, ok := s.opts.clientIdentities[pk]
	if !ok {
		log.Print("Closing connection, client uses unknown public key")
		return
	}

	handleAuthenticatedClient(conn, name)
}

func handleAuthenticatedClient(conn net.Conn, name string) {
	for {
		buf := make([]byte, 128)
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Error ocurred while reading: %v", err)
			return
		} else {
			log.Printf("Received from: %v data: %v", name, string(buf[:n]))
		}
	}
}
