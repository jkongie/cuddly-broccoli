package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	pb "github.com/smartcontractkit/sync/sync"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

type server struct {
	pb.UnimplementedSyncServer
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSyncServer(grpcServer, &server{})
	grpcServer.Serve(lis)
}

// ServerStream receives a stream from the client which it will use to push a
// timestamp
func (s *server) ServerStream(stream pb.Sync_ServerStreamServer) error {
	fmt.Println("Initializing Server Stream")

	// Set up sending
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	done := make(chan struct{})

	go func() {
		for {
			select {
			case t := <-ticker.C:
				if err := stream.Send(&pb.Request{Message: fmt.Sprintf("Server Message: %d", t.Unix())}); err != nil {
					log.Fatalf("Failed to send a message: %v", err)
				}
			case <-done:
				return
			}
		}
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			close(done)
			fmt.Println("Stream closed")
			return err
		}
		if err != nil {
			fmt.Println(err)
			return nil
		}

		// Process the message here
		fmt.Println(in.GetMessage())
	}
}
