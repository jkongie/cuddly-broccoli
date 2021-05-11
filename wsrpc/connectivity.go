// Package connectivity defines connectivity semantics.
package wsrpc

import (
	"log"
	"sync"
)

// State indicates the state of connectivity.
// It can be the state of a ClientConn or SubConn.
type ConnectivityState int

func (s ConnectivityState) String() string {
	switch s {
	case ConnectivityStateIdle:
		return "IDLE"
	case ConnectivityStateConnecting:
		return "CONNECTING"
	case ConnectivityStateReady:
		return "READY"
	case ConnectivityStateTransientFailure:
		return "TRANSIENT_FAILURE"
	case ConnectivityStateShutdown:
		return "SHUTDOWN"
	default:
		log.Printf("unknown connectivity state: %d", s)
		return "Invalid-State"
	}
}

const (
	// Idle indicates the ClientConn is idle.
	ConnectivityStateIdle ConnectivityState = iota
	// Connecting indicates the ClientConn is connecting.
	ConnectivityStateConnecting
	// Ready indicates the ClientConn is ready for work.
	ConnectivityStateReady
	// TransientFailure indicates the ClientConn has seen a failure but expects to recover.
	ConnectivityStateTransientFailure
	// Shutdown indicates the ClientConn has started shutting down.
	ConnectivityStateShutdown
)

// connectivityStateManager keeps the connectivity.State of ClientConn.
// This struct will eventually be exported so the balancers can access it.
type connectivityStateManager struct {
	mu    sync.Mutex
	state ConnectivityState
	// notifyChan chan struct{}
	// channelzID int64
}

// updateState updates the connectivity.State of ClientConn.
// If there's a change it notifies goroutines waiting on state change to
// happen.
func (csm *connectivityStateManager) updateState(state ConnectivityState) {
	csm.mu.Lock()
	defer csm.mu.Unlock()
	if csm.state == ConnectivityStateShutdown {
		return
	}
	if csm.state == state {
		return
	}
	csm.state = state
	// channelz.Infof(logger, csm.channelzID, "Channel Connectivity change to %v", state)
	// if csm.notifyChan != nil {
	// 	// There are other goroutines waiting on this channel.
	// 	close(csm.notifyChan)
	// 	csm.notifyChan = nil
	// }
}

func (csm *connectivityStateManager) getState() ConnectivityState {
	csm.mu.Lock()
	defer csm.mu.Unlock()
	return csm.state
}

// func (csm *connectivityStateManager) getNotifyChan() <-chan struct{} {
// 	csm.mu.Lock()
// 	defer csm.mu.Unlock()
// 	if csm.notifyChan == nil {
// 		csm.notifyChan = make(chan struct{})
// 	}
// 	return csm.notifyChan
// }
