// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             (unknown)
// source: api/v1/frontmatter.proto

package apiv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	Frontmatter_GetFrontmatter_FullMethodName     = "/api.v1.Frontmatter/GetFrontmatter"
	Frontmatter_MergeFrontmatter_FullMethodName   = "/api.v1.Frontmatter/MergeFrontmatter"
	Frontmatter_ReplaceFrontmatter_FullMethodName = "/api.v1.Frontmatter/ReplaceFrontmatter"
	Frontmatter_RemoveKeyAtPath_FullMethodName    = "/api.v1.Frontmatter/RemoveKeyAtPath"
)

// FrontmatterClient is the client API for Frontmatter service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// The Frontmatter service definition.
type FrontmatterClient interface {
	// Gets the frontmatter for a given page.
	GetFrontmatter(ctx context.Context, in *GetFrontmatterRequest, opts ...grpc.CallOption) (*GetFrontmatterResponse, error)
	// Merges the given frontmatter with the existing frontmatter for a page.
	MergeFrontmatter(ctx context.Context, in *MergeFrontmatterRequest, opts ...grpc.CallOption) (*MergeFrontmatterResponse, error)
	// Replaces the entire frontmatter for a given page.
	ReplaceFrontmatter(ctx context.Context, in *ReplaceFrontmatterRequest, opts ...grpc.CallOption) (*ReplaceFrontmatterResponse, error)
	// Removes a key from the frontmatter at a given path.
	RemoveKeyAtPath(ctx context.Context, in *RemoveKeyAtPathRequest, opts ...grpc.CallOption) (*RemoveKeyAtPathResponse, error)
}

type frontmatterClient struct {
	cc grpc.ClientConnInterface
}

func NewFrontmatterClient(cc grpc.ClientConnInterface) FrontmatterClient {
	return &frontmatterClient{cc}
}

func (c *frontmatterClient) GetFrontmatter(ctx context.Context, in *GetFrontmatterRequest, opts ...grpc.CallOption) (*GetFrontmatterResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetFrontmatterResponse)
	err := c.cc.Invoke(ctx, Frontmatter_GetFrontmatter_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *frontmatterClient) MergeFrontmatter(ctx context.Context, in *MergeFrontmatterRequest, opts ...grpc.CallOption) (*MergeFrontmatterResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(MergeFrontmatterResponse)
	err := c.cc.Invoke(ctx, Frontmatter_MergeFrontmatter_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *frontmatterClient) ReplaceFrontmatter(ctx context.Context, in *ReplaceFrontmatterRequest, opts ...grpc.CallOption) (*ReplaceFrontmatterResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ReplaceFrontmatterResponse)
	err := c.cc.Invoke(ctx, Frontmatter_ReplaceFrontmatter_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *frontmatterClient) RemoveKeyAtPath(ctx context.Context, in *RemoveKeyAtPathRequest, opts ...grpc.CallOption) (*RemoveKeyAtPathResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RemoveKeyAtPathResponse)
	err := c.cc.Invoke(ctx, Frontmatter_RemoveKeyAtPath_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FrontmatterServer is the server API for Frontmatter service.
// All implementations must embed UnimplementedFrontmatterServer
// for forward compatibility
//
// The Frontmatter service definition.
type FrontmatterServer interface {
	// Gets the frontmatter for a given page.
	GetFrontmatter(context.Context, *GetFrontmatterRequest) (*GetFrontmatterResponse, error)
	// Merges the given frontmatter with the existing frontmatter for a page.
	MergeFrontmatter(context.Context, *MergeFrontmatterRequest) (*MergeFrontmatterResponse, error)
	// Replaces the entire frontmatter for a given page.
	ReplaceFrontmatter(context.Context, *ReplaceFrontmatterRequest) (*ReplaceFrontmatterResponse, error)
	// Removes a key from the frontmatter at a given path.
	RemoveKeyAtPath(context.Context, *RemoveKeyAtPathRequest) (*RemoveKeyAtPathResponse, error)
	mustEmbedUnimplementedFrontmatterServer()
}

// UnimplementedFrontmatterServer must be embedded to have forward compatible implementations.
type UnimplementedFrontmatterServer struct {
}

func (UnimplementedFrontmatterServer) GetFrontmatter(context.Context, *GetFrontmatterRequest) (*GetFrontmatterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetFrontmatter not implemented")
}
func (UnimplementedFrontmatterServer) MergeFrontmatter(context.Context, *MergeFrontmatterRequest) (*MergeFrontmatterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MergeFrontmatter not implemented")
}
func (UnimplementedFrontmatterServer) ReplaceFrontmatter(context.Context, *ReplaceFrontmatterRequest) (*ReplaceFrontmatterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReplaceFrontmatter not implemented")
}
func (UnimplementedFrontmatterServer) RemoveKeyAtPath(context.Context, *RemoveKeyAtPathRequest) (*RemoveKeyAtPathResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveKeyAtPath not implemented")
}
func (UnimplementedFrontmatterServer) mustEmbedUnimplementedFrontmatterServer() {}

// UnsafeFrontmatterServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FrontmatterServer will
// result in compilation errors.
type UnsafeFrontmatterServer interface {
	mustEmbedUnimplementedFrontmatterServer()
}

func RegisterFrontmatterServer(s grpc.ServiceRegistrar, srv FrontmatterServer) {
	s.RegisterService(&Frontmatter_ServiceDesc, srv)
}

func _Frontmatter_GetFrontmatter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetFrontmatterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FrontmatterServer).GetFrontmatter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Frontmatter_GetFrontmatter_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FrontmatterServer).GetFrontmatter(ctx, req.(*GetFrontmatterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Frontmatter_MergeFrontmatter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MergeFrontmatterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FrontmatterServer).MergeFrontmatter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Frontmatter_MergeFrontmatter_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FrontmatterServer).MergeFrontmatter(ctx, req.(*MergeFrontmatterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Frontmatter_ReplaceFrontmatter_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReplaceFrontmatterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FrontmatterServer).ReplaceFrontmatter(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Frontmatter_ReplaceFrontmatter_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FrontmatterServer).ReplaceFrontmatter(ctx, req.(*ReplaceFrontmatterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Frontmatter_RemoveKeyAtPath_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RemoveKeyAtPathRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FrontmatterServer).RemoveKeyAtPath(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Frontmatter_RemoveKeyAtPath_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FrontmatterServer).RemoveKeyAtPath(ctx, req.(*RemoveKeyAtPathRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Frontmatter_ServiceDesc is the grpc.ServiceDesc for Frontmatter service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Frontmatter_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.v1.Frontmatter",
	HandlerType: (*FrontmatterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetFrontmatter",
			Handler:    _Frontmatter_GetFrontmatter_Handler,
		},
		{
			MethodName: "MergeFrontmatter",
			Handler:    _Frontmatter_MergeFrontmatter_Handler,
		},
		{
			MethodName: "ReplaceFrontmatter",
			Handler:    _Frontmatter_ReplaceFrontmatter_Handler,
		},
		{
			MethodName: "RemoveKeyAtPath",
			Handler:    _Frontmatter_RemoveKeyAtPath_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/v1/frontmatter.proto",
}
