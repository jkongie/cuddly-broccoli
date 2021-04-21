// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.1.0
// - protoc             v3.15.8
// source: sync/sync.proto

package sync

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SyncClient is the client API for Sync service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SyncClient interface {
	// A Bidirectional streaming RPC.
	ServerStream(ctx context.Context, opts ...grpc.CallOption) (Sync_ServerStreamClient, error)
}

type syncClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncClient(cc grpc.ClientConnInterface) SyncClient {
	return &syncClient{cc}
}

func (c *syncClient) ServerStream(ctx context.Context, opts ...grpc.CallOption) (Sync_ServerStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &Sync_ServiceDesc.Streams[0], "/sync.Sync/ServerStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &syncServerStreamClient{stream}
	return x, nil
}

type Sync_ServerStreamClient interface {
	Send(*Request) error
	Recv() (*Request, error)
	grpc.ClientStream
}

type syncServerStreamClient struct {
	grpc.ClientStream
}

func (x *syncServerStreamClient) Send(m *Request) error {
	return x.ClientStream.SendMsg(m)
}

func (x *syncServerStreamClient) Recv() (*Request, error) {
	m := new(Request)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// SyncServer is the server API for Sync service.
// All implementations must embed UnimplementedSyncServer
// for forward compatibility
type SyncServer interface {
	// A Bidirectional streaming RPC.
	ServerStream(Sync_ServerStreamServer) error
	mustEmbedUnimplementedSyncServer()
}

// UnimplementedSyncServer must be embedded to have forward compatible implementations.
type UnimplementedSyncServer struct {
}

func (UnimplementedSyncServer) ServerStream(Sync_ServerStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method ServerStream not implemented")
}
func (UnimplementedSyncServer) mustEmbedUnimplementedSyncServer() {}

// UnsafeSyncServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SyncServer will
// result in compilation errors.
type UnsafeSyncServer interface {
	mustEmbedUnimplementedSyncServer()
}

func RegisterSyncServer(s grpc.ServiceRegistrar, srv SyncServer) {
	s.RegisterService(&Sync_ServiceDesc, srv)
}

func _Sync_ServerStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(SyncServer).ServerStream(&syncServerStreamServer{stream})
}

type Sync_ServerStreamServer interface {
	Send(*Request) error
	Recv() (*Request, error)
	grpc.ServerStream
}

type syncServerStreamServer struct {
	grpc.ServerStream
}

func (x *syncServerStreamServer) Send(m *Request) error {
	return x.ServerStream.SendMsg(m)
}

func (x *syncServerStreamServer) Recv() (*Request, error) {
	m := new(Request)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Sync_ServiceDesc is the grpc.ServiceDesc for Sync service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Sync_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sync.Sync",
	HandlerType: (*SyncServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ServerStream",
			Handler:       _Sync_ServerStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "sync/sync.proto",
}
