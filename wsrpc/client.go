package wsrpc

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log"
	"sync"
)

var (
	// errConnClosing indicates that the connection is closing.
	errConnClosing = errors.New("grpc: the connection is closing")
)

// ClientConn represents a virtual connection to a conceptual endpoint, to
// perform RPCs.
//
type ClientConn struct {
	ctx context.Context
	// cancel context.CancelFunc

	mu sync.RWMutex

	target string

	// Manage connection state - Needed?
	csMgr *connectivityStateManager
	csCh  <-chan ConnectivityState

	dopts dialOptions
	conn  *addrConn

	readFn func(message []byte)
}

// dialOptions configure a Dial call. dialOptions are set by the DialOption
// values passed to Dial.
type dialOptions struct {
	copts        ConnectOptions
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
		o.copts.TransportCredentials = NewTransportCredentials(
			privKey,
			map[[ed25519.PublicKeySize]byte]string{
				serverPubKey: "server",
			},
		)
	})
}

func defaultDialOptions() dialOptions {
	return dialOptions{
		copts: ConnectOptions{
			// 	WriteBufferSize: defaultWriteBufSize,
			// 	ReadBufferSize:  defaultReadBufSize,
		},
	}
}

func Dial(target string, opts ...DialOption) (*ClientConn, error) {
	cc := &ClientConn{
		ctx:    context.Background(),
		target: target,
		csMgr:  &connectivityStateManager{},
		dopts:  defaultDialOptions(),
	}

	for _, opt := range opts {
		opt.apply(&cc.dopts)
	}

	addrConn, err := cc.newAddrConn(target)
	if err != nil {
		return nil, errors.New("Could not establish a connection")
	}

	addrConn.connect()
	cc.conn = addrConn

	return cc, nil
}

// newAddrConn creates an addrConn for addrs and adds it to cc.conns.
func (cc *ClientConn) newAddrConn(addr string) (*addrConn, error) {
	csCh := make(chan ConnectivityState)
	ac := &addrConn{
		state:   ConnectivityStateIdle,
		stateCh: csCh,
		cc:      cc,
		addr:    addr,
		dopts:   cc.dopts,

		// resetBackoff: make(chan struct{}),
	}
	// ac.ctx, ac.cancel = context.WithCancel(cc.ctx)
	// Track ac in cc. This needs to be done before any getTransport(...) is called.
	cc.mu.Lock()
	// if cc.conn == nil {
	// 	cc.mu.Unlock()
	// 	return nil, ErrClientConnClosing
	// }

	cc.conn = ac
	cc.csCh = csCh
	cc.mu.Unlock()

	// Register handlers when the state changes
	go cc.listen()

	return ac, nil
}

// listen for the connectivty state to be ready and enable the handler
// TODO - Shutdown the routine during when the client is closed
func (cc *ClientConn) listen() {
	for {
		s := <-cc.csCh

		var done chan struct{}

		if s == ConnectivityStateReady {
			done := make(chan struct{})
			go cc.startRead(done)
		} else {
			if done != nil {
				close(done)
			}
		}
	}
}

func (cc *ClientConn) startRead(done <-chan struct{}) {
	for {
		select {

		case msg := <-cc.conn.transport.Read():
			cc.readFn(msg)
		case <-done:
			return
		}
	}
}

// GetState returns the connectivity.State of ClientConn.
func (cc *ClientConn) GetState() ConnectivityState {
	return cc.csMgr.getState()
}

// Close tears down the ClientConn and all underlying connections.
func (cc *ClientConn) Close() {
	log.Println("Closing client connection")

	conn := cc.conn

	cc.conn = nil
	cc.csMgr.updateState(ConnectivityStateShutdown)

	// Close the websocket connection
	// Take another look at this. We may not want to do this here.
	// We could inspect shutdown state
	if conn.transport != nil {
		conn.transport.Close()
	}
}

// TODO - Figure out a way to return errors
func (cc *ClientConn) Send(message string) error {
	if cc.conn.state != ConnectivityStateReady {
		return errors.New("connection is not ready")
	}

	cc.conn.transport.Write([]byte(message))

	return nil
}

// func (cc *ClientConn) Receive(handler func(message []byte)) {
// 	cc.conn.transport.Read(handler)
// }

func (cc *ClientConn) RegisterHandler(handler func(message []byte)) {
	cc.readFn = handler
}

// addrConn is a network connection to a given address.
type addrConn struct {
	cc *ClientConn

	addr  string
	dopts dialOptions

	// transport is set when there's a viable transport, and is reset
	// to nil when the current transport should no longer be used (e.g.
	// after transport is closed, ac has been torn down).
	transport ClientTransport // The current transport.

	mu sync.Mutex

	// Use updateConnectivityState for updating addrConn's connectivity state.
	state ConnectivityState
	// Notifies this channel when the ConnectivityState changes
	stateCh chan ConnectivityState

	// resetBackoff chan struct{}
}

func (ac *addrConn) connect() error {
	ac.mu.Lock()
	if ac.state == ConnectivityStateShutdown {
		ac.mu.Unlock()
		return errConnClosing
	}

	if ac.state != ConnectivityStateIdle {
		ac.mu.Unlock()
		return nil
	}

	// Update connectivity state within the lock to prevent subsequent or
	// concurrent calls from resetting the transport more than once.
	ac.updateConnectivityState(ConnectivityStateConnecting, nil)
	ac.mu.Unlock()

	// Start a goroutine connecting to the server asynchronously.
	go ac.resetTransport()

	return nil
}

// Note: this requires a lock on ac.mu.
func (ac *addrConn) updateConnectivityState(s ConnectivityState, lastErr error) {
	if ac.state == s {
		return
	}
	ac.state = s
	ac.stateCh <- s
	fmt.Println("Set Connectivity State to ", s)
}

func (ac *addrConn) resetTransport() {
	ac.mu.Lock()
	if ac.state == ConnectivityStateShutdown {
		ac.mu.Unlock()
		return
	}

	// addrs := ac.addrs
	// backoffFor := ac.dopts.bs.Backoff(ac.backoffIdx)
	// // This will be the duration that dial gets to finish.
	// dialDuration := minConnectTimeout
	// if ac.dopts.minConnectTimeout != nil {
	// 	dialDuration = ac.dopts.minConnectTimeout()
	// }

	addr := ac.addr
	copts := ac.dopts.copts

	ac.updateConnectivityState(ConnectivityStateConnecting, nil)
	ac.transport = nil
	ac.mu.Unlock()

	newTr, err := ac.createTransport(addr, copts)
	if err != nil {
		// After connection failure, the addrConn enters TRANSIENT_FAILURE.
		ac.mu.Lock()
		if ac.state == ConnectivityStateShutdown {
			ac.mu.Unlock()
			return
		}
		ac.updateConnectivityState(ConnectivityStateTransientFailure, err)

		// Backoff.
		// b := ac.resetBackoff
		ac.mu.Unlock()

		// timer := time.NewTimer(backoffFor)
		// select {
		// case <-timer.C:
		// 	ac.mu.Lock()
		// 	ac.backoffIdx++
		// 	ac.mu.Unlock()
		// case <-b:
		// 	timer.Stop()
		// case <-ac.ctx.Done():
		// 	timer.Stop()
		// 	return
		// }
		// continue

		return
	}

	// Close the transport if in a shutdown state
	ac.mu.Lock()
	if ac.state == ConnectivityStateShutdown {
		ac.mu.Unlock()
		newTr.Close()
		return
	}
	ac.transport = newTr
	ac.updateConnectivityState(ConnectivityStateReady, nil)

	return

	// TODO
	// Block until the created transport is down. And when this happens,
	// we restart from the top of the addr list.
	// <-reconnect.Done()
	// hcancel()
	// restart connecting - the top of the loop will set state to
	// CONNECTING.  This is against the current connectivity semantics doc,
	// however it allows for graceful behavior for RPCs not yet dispatched
	// - unfortunate timing would otherwise lead to the RPC failing even
	// though the TRANSIENT_FAILURE state (called for by the doc) would be
	// instantaneous.
	//
	// Ideally we should transition to Idle here and block until there is
	// RPC activity that leads to the balancer requesting a reconnect of
	// the associated SubConn.

}

func (ac *addrConn) createTransport(addr string, copts ConnectOptions) (ClientTransport, error) {
	return NewWebsocketClient(ac.cc.ctx, addr, copts)
}
