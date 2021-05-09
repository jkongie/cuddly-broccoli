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
	"google.golang.org/grpc/metadata"
)

var (
	port = flag.Int("port", 10000, "The server port")
)

type RequestHandler struct {
	AccessKey string
}

func (h *RequestHandler) Handle(in *pb.Request) {
	fmt.Println(h.AccessKey, in.GetMessage())
}

type NodeConnection struct {
	done   chan struct{}
	C      chan string
	ticker *time.Ticker
}

func NewNodeConnection() NodeConnection {
	return NodeConnection{
		done:   make(chan struct{}),
		C:      make(chan string, 1),
		ticker: time.NewTicker(1 * time.Second),
	}
}

func (c NodeConnection) Start() {
	go func() {
		for {
			select {
			case t := <-c.ticker.C:
				c.C <- fmt.Sprintf("Server Message: %d", t.Unix())
			case <-c.done:
				return
			}
		}
	}()
}

func (c NodeConnection) Stop() {
	close(c.done)
	c.ticker.Stop()
}

// func RegisterSendQueue(accessKey string) <-chan string {
// 	ch := make(chan string, 1)
// 	queues[accessKey] = ch
// 	ch <- "ping"

// 	return ch
// }
type server struct {
	pb.UnimplementedSyncServer

	requestHandlers map[string]RequestHandler
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSyncServer(grpcServer, &server{requestHandlers: map[string]RequestHandler{}})
	grpcServer.Serve(lis)
}

// ServerStream receives a stream from the client which it will use to push a
// timestamp
func (s *server) ServerStream(stream pb.Sync_ServerStreamServer) error {
	fmt.Println("Initializing Server Stream")
	fmt.Println("Current RequestHandlers", s.requestHandlers)

	// header, err := stream.Header()
	md, _ := metadata.FromIncomingContext(stream.Context())
	fmt.Println("AccessKey:", md["accesskey"])

	accessKey := md["accesskey"]
	// fmt.Println("ID: ", stream.Context().Value("id"))
	s.requestHandlers[accessKey[0]] = RequestHandler{AccessKey: accessKey[0]}
	defer func() {
		delete(s.requestHandlers, accessKey[0])
	}()

	nc := NewNodeConnection()
	nc.Start()
	defer nc.Stop()

	done := make(chan struct{})

	pings := make(chan string, 1)
	pings <- "ping"

	go func() {
		for {
			select {
			case msg := <-nc.C:
				if err := stream.Send(&pb.Request{Message: msg}); err != nil {
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

		handler := s.requestHandlers[accessKey[0]]

		handler.Handle(in)
	}
}
