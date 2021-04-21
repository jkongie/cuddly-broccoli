package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	pb "github.com/smartcontractkit/sync/sync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	serverAddr = flag.String("server_addr", "localhost:10000", "The server address in the format of host:port")
)

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())

	// Connect to the GRPC service
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewSyncClient(conn)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				fmt.Println("Opening Stream")
				runSync(client, done)
			}

		}
	}()

	<-interrupt
	close(done)
}

// runSync establishes a bidirectional stream to the server
func runSync(client pb.SyncClient, done chan struct{}) {
	ctx := context.Background()

	// WaitForReady waits until the connection is reestablished before it
	// initializes a stream
	stream, err := client.ServerStream(ctx, grpc.WaitForReady(true))
	if err != nil {
		log.Printf("%v.Sync(_) = _, %v", client, err)

		return
	}

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("Server closed")

				return
			}
			if err != nil {
				if st, ok := status.FromError(err); ok {
					// Exit if the server is no longer running
					if st.Code() == codes.Unavailable {
						return
					}

				}

				// Print an error for any other error and continue
				fmt.Println("Failed to receive a message", err)

				break
			}
			log.Printf("Got message %s", in.Message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Send messages
	for {
		select {
		case t := <-ticker.C:
			err := stream.Send(&pb.Request{Message: fmt.Sprintf("Client Message: %d", t.Unix())})
			if err == io.EOF {
				fmt.Println("Server closed")

				return
			}
			if err != nil {
				log.Printf("Failed to send a message: %v", err)
			}
		case <-done:
			stream.CloseSend()
			return
		}
	}
}
