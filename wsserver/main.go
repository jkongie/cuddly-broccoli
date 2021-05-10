package main

import (
	"crypto/ed25519"
	"errors"
	"log"
	"net"
	"time"

	"github.com/smartcontractkit/sync/keys"
	"github.com/smartcontractkit/sync/wsrpc"
)

func main() {
	privKey := keys.FromHex(keys.ServerPrivKey)

	clientIdentities := map[[ed25519.PublicKeySize]byte]string{}
	for _, c := range keys.Clients {
		if c.RegisteredOnServer {
			clientPubKey := keys.FromHex(c.PubKey)

			staticClientPubKey, err := keys.ToStaticSizedBytes(clientPubKey)
			if err != nil {
				panic(err)
			}

			clientIdentities[staticClientPubKey] = c.Name
		}
	}

	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := wsrpc.NewServer(wsrpc.Creds(privKey, clientIdentities))
	go s.Serve(lis)

	go receiveMessages(s.Broadcast())
	go sendMessages(s, clientIdentities)

	select {}
}

func receiveMessages(ch <-chan []byte) {
	for {
		message := <-ch
		log.Printf("received: %s", message)
	}
}

// Sends messages to all registered clients. Clients may not have an active
// connection.
func sendMessages(s *wsrpc.Server, clientIdentities map[[ed25519.PublicKeySize]byte]string) {
	for {
		for pubKey, name := range clientIdentities {
			err := s.Send(pubKey, []byte("Pong"))
			if err != nil {
				if errors.Is(err, wsrpc.ErrNotConnected) {
					log.Printf("%s: %v", name, err)
				} else {
					log.Printf("Some error ocurred ponging: %v", err)
				}

				continue
			}

			log.Printf("Sent: Pong to %s", name)
		}

		time.Sleep(5 * time.Second)
	}
}
