package wsrpc

import (
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

type ClientConn struct {
	// ctx    context.Context
	// cancel context.CancelFunc

	target string
	// parsedTarget resolver.Target
	// authority    string
	dopts dialOptions
	// csMgr        *connectivityStateManager

	// balancerBuildOpts balancer.BuildOptions
	// blockingpicker    *pickerWrapper

	// safeConfigSelector iresolver.SafeConfigSelector

	// mu              sync.RWMutex
	// resolverWrapper *ccResolverWrapper
	// sc              *ServiceConfig
	// conns           map[*addrConn]struct{}
	// Keepalive parameter can be updated if a GoAway is received.
	// mkp             keepalive.ClientParameters
	// curBalancerName string
	// balancerWrapper *ccBalancerWrapper
	// retryThrottler  atomic.Value

	// firstResolveEvent *grpcsync.Event

	// channelzID int64 // channelz unique identification number
	// czData     *channelzData

	// lceMu               sync.Mutex // protects lastConnectionError
	// lastConnectionError error
}

// dialOptions configure a Dial call. dialOptions are set by the DialOption
// values passed to Dial.
type dialOptions struct {
	privKey      ed25519.PrivateKey
	serverPubKey [ed25519.PublicKeySize]byte

	// chainUnaryInts  []UnaryClientInterceptor
	// chainStreamInts []StreamClientInterceptor

	// cp              Compressor
	// dc              Decompressor
	// bs              internalbackoff.Strategy
	// block           bool
	// returnLastError bool
	// insecure        bool
	// timeout         time.Duration
	// scChan          <-chan ServiceConfig
	// authority       string
	// copts           transport.ConnectOptions
	// callOptions     []CallOption
	// // This is used by WithBalancerName dial option.
	// balancerBuilder             balancer.Builder
	// channelzParentID            int64
	// disableServiceConfig        bool
	// disableRetry                bool
	// disableHealthCheck          bool
	// healthCheckFunc             internal.HealthChecker
	// minConnectTimeout           func() time.Duration
	// defaultServiceConfig        *ServiceConfig // defaultServiceConfig is parsed from defaultServiceConfigRawJSON.
	// defaultServiceConfigRawJSON *string
	// // This is used by ccResolverWrapper to backoff between successive calls to
	// // resolver.ResolveNow(). The user will have no need to configure this, but
	// // we need to be able to configure this in tests.
	// resolveNowBackoff func(int) time.Duration
	// resolvers         []resolver.Builder
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
		// disableRetry:    !envconfig.Retry,
		// healthCheckFunc: internal.HealthCheckFunc,
		// copts: transport.ConnectOptions{
		// 	WriteBufferSize: defaultWriteBufSize,
		// 	ReadBufferSize:  defaultReadBufSize,
		// 	UseProxy:        true,
		// },
		// resolveNowBackoff: internalbackoff.DefaultExponential.Backoff,
	}
}

func Dial(target string, opts ...DialOption) (conn *ClientConn, err error) {
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

	tlsConn, err := tls.Dial("tcp", target, &config)
	if err != nil {
		log.Println("error")
		log.Println(err)
		return
	}

	handleConnection(tlsConn)

	return cc, nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	for {
		n, err := conn.Write([]byte("Ping"))
		if n != 4 || err != nil {
			log.Printf("Some error ocurred pinging: %v %v", n, err)
		} else {
			log.Println("Sent: Ping")
		}

		time.Sleep(5 * time.Second)
	}
}
