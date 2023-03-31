// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: internal/messaging/rusi/v1/rusi.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// RusiClient is the client API for Rusi service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RusiClient interface {
	// Publishes events to the specific topic.
	Publish(ctx context.Context, in *PublishRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// Subscribe pushes events on the stream
	Subscribe(ctx context.Context, opts ...grpc.CallOption) (Rusi_SubscribeClient, error)
}

type rusiClient struct {
	cc grpc.ClientConnInterface
}

func NewRusiClient(cc grpc.ClientConnInterface) RusiClient {
	return &rusiClient{cc}
}

func (c *rusiClient) Publish(ctx context.Context, in *PublishRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/rusi.proto.runtime.v1.Rusi/Publish", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rusiClient) Subscribe(ctx context.Context, opts ...grpc.CallOption) (Rusi_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &Rusi_ServiceDesc.Streams[0], "/rusi.proto.runtime.v1.Rusi/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &rusiSubscribeClient{stream}
	return x, nil
}

type Rusi_SubscribeClient interface {
	Send(*SubscribeRequest) error
	Recv() (*ReceivedMessage, error)
	grpc.ClientStream
}

type rusiSubscribeClient struct {
	grpc.ClientStream
}

func (x *rusiSubscribeClient) Send(m *SubscribeRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *rusiSubscribeClient) Recv() (*ReceivedMessage, error) {
	m := new(ReceivedMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// RusiServer is the server API for Rusi service.
// All implementations must embed UnimplementedRusiServer
// for forward compatibility
type RusiServer interface {
	// Publishes events to the specific topic.
	Publish(context.Context, *PublishRequest) (*emptypb.Empty, error)
	// Subscribe pushes events on the stream
	Subscribe(Rusi_SubscribeServer) error
	mustEmbedUnimplementedRusiServer()
}

// UnimplementedRusiServer must be embedded to have forward compatible implementations.
type UnimplementedRusiServer struct {
}

func (UnimplementedRusiServer) Publish(context.Context, *PublishRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Publish not implemented")
}
func (UnimplementedRusiServer) Subscribe(Rusi_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedRusiServer) mustEmbedUnimplementedRusiServer() {}

// UnsafeRusiServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RusiServer will
// result in compilation errors.
type UnsafeRusiServer interface {
	mustEmbedUnimplementedRusiServer()
}

func RegisterRusiServer(s grpc.ServiceRegistrar, srv RusiServer) {
	s.RegisterService(&Rusi_ServiceDesc, srv)
}

func _Rusi_Publish_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PublishRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RusiServer).Publish(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rusi.proto.runtime.v1.Rusi/Publish",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RusiServer).Publish(ctx, req.(*PublishRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Rusi_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RusiServer).Subscribe(&rusiSubscribeServer{stream})
}

type Rusi_SubscribeServer interface {
	Send(*ReceivedMessage) error
	Recv() (*SubscribeRequest, error)
	grpc.ServerStream
}

type rusiSubscribeServer struct {
	grpc.ServerStream
}

func (x *rusiSubscribeServer) Send(m *ReceivedMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *rusiSubscribeServer) Recv() (*SubscribeRequest, error) {
	m := new(SubscribeRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Rusi_ServiceDesc is the grpc.ServiceDesc for Rusi service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Rusi_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "rusi.proto.runtime.v1.Rusi",
	HandlerType: (*RusiServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Publish",
			Handler:    _Rusi_Publish_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _Rusi_Subscribe_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "internal/messaging/rusi/v1/rusi.proto",
}