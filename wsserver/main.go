package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"net"

	"github.com/smartcontractkit/sync/keys"
	"github.com/smartcontractkit/sync/wsrpc"
)

func main() {
	privKey := make([]byte, hex.DecodedLen(len(keys.ServerPrivKey)))
	hex.Decode(privKey, []byte(keys.ServerPrivKey))

	clientIdentities := map[[ed25519.PublicKeySize]byte]string{}
	for _, c := range keys.Clients {
		if c.RegisteredOnServer {
			clientPubKey := make([]byte, hex.DecodedLen(len(c.PubKey)))
			hex.Decode(clientPubKey, []byte(c.PubKey))

			// Copy the pub key into a statically sized byte array
			var staticClientPubKey [ed25519.PublicKeySize]byte
			if ed25519.PublicKeySize != copy(staticClientPubKey[:], clientPubKey) {
				// assertion
				panic("copying public key failed")
			}

			clientIdentities[staticClientPubKey] = c.Name
		}
	}

	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := wsrpc.NewServer(wsrpc.Creds(privKey, clientIdentities))
	s.Serve(lis)
}
