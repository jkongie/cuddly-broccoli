package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/smartcontractkit/sync/keys"
	"github.com/smartcontractkit/sync/wsrpc"
)

func main() {
	if len(os.Args[1:]) == 0 {
		log.Fatalf("Must provide the index of the client you wish to run")
	}
	// Run the client matching the array index
	arg1 := os.Args[1]

	cidx, err := strconv.Atoi(arg1)
	if err != nil {
		log.Fatalf("arg must be an int")
	}

	client := keys.Clients[cidx]

	privKey := make([]byte, hex.DecodedLen(len(client.PrivKey)))
	hex.Decode(privKey, []byte(client.PrivKey))

	serverPubKey := make([]byte, ed25519.PublicKeySize)
	hex.Decode(serverPubKey, []byte(keys.ServerPubKey))

	// Copy the pub key into a statically sized byte array
	var pubStaticServer [ed25519.PublicKeySize]byte
	if ed25519.PublicKeySize != copy(pubStaticServer[:], serverPubKey) {
		// assertion
		panic("copying public key failed")
	}

	cl, err := wsrpc.Dial("127.0.0.1:1337", wsrpc.WithTransportCreds(privKey, pubStaticServer))
	if err != nil {
		log.Fatalln(err)
	}
	defer cl.CloseConn()

	go writeClientWS(cl)
	go readClientWS(cl)

	select {}
}

func writeClientWS(c *wsrpc.Client) {
	for {
		err := c.Send("Ping")
		if err != nil {
			log.Printf("Some error ocurred pinging: %v", err)
			return
		}

		log.Println("Sent: Ping")

		time.Sleep(5 * time.Second)
	}
}

func readClientWS(c *wsrpc.Client) {
	ch := make(chan string)
	c.Receive(ch)

	for {
		message := <-ch

		log.Printf("recv: %s", message)
	}
}
