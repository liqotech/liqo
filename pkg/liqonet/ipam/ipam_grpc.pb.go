// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package ipam

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

// IpamClient is the client API for Ipam service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IpamClient interface {
	MapEndpointIP(ctx context.Context, in *MapRequest, opts ...grpc.CallOption) (*MapResponse, error)
	UnmapEndpointIP(ctx context.Context, in *UnmapRequest, opts ...grpc.CallOption) (*UnmapResponse, error)
	GetHomePodIP(ctx context.Context, in *GetHomePodIPRequest, opts ...grpc.CallOption) (*GetHomePodIPResponse, error)
	BelongsToPodCIDR(ctx context.Context, in *BelongsRequest, opts ...grpc.CallOption) (*BelongsResponse, error)
	GetClusterIdentity(ctx context.Context, in *ClusterIdentityRequest, opts ...grpc.CallOption) (*ClusterIdentityResponse, error)
	DoesClusterMappingExist(ctx context.Context, in *ClusterMappingRequest, opts ...grpc.CallOption) (*ClusterMappingResponse, error)
}

type ipamClient struct {
	cc grpc.ClientConnInterface
}

func NewIpamClient(cc grpc.ClientConnInterface) IpamClient {
	return &ipamClient{cc}
}

func (c *ipamClient) MapEndpointIP(ctx context.Context, in *MapRequest, opts ...grpc.CallOption) (*MapResponse, error) {
	out := new(MapResponse)
	err := c.cc.Invoke(ctx, "/ipam/MapEndpointIP", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) UnmapEndpointIP(ctx context.Context, in *UnmapRequest, opts ...grpc.CallOption) (*UnmapResponse, error) {
	out := new(UnmapResponse)
	err := c.cc.Invoke(ctx, "/ipam/UnmapEndpointIP", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) GetHomePodIP(ctx context.Context, in *GetHomePodIPRequest, opts ...grpc.CallOption) (*GetHomePodIPResponse, error) {
	out := new(GetHomePodIPResponse)
	err := c.cc.Invoke(ctx, "/ipam/GetHomePodIP", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) BelongsToPodCIDR(ctx context.Context, in *BelongsRequest, opts ...grpc.CallOption) (*BelongsResponse, error) {
	out := new(BelongsResponse)
	err := c.cc.Invoke(ctx, "/ipam/BelongsToPodCIDR", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) GetClusterIdentity(ctx context.Context, in *ClusterIdentityRequest, opts ...grpc.CallOption) (*ClusterIdentityResponse, error) {
	out := new(ClusterIdentityResponse)
	err := c.cc.Invoke(ctx, "/ipam/GetClusterIdentity", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) DoesClusterMappingExist(ctx context.Context, in *ClusterMappingRequest, opts ...grpc.CallOption) (*ClusterMappingResponse, error) {
	out := new(ClusterMappingResponse)
	err := c.cc.Invoke(ctx, "/ipam/DoesClusterMappingExist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// IpamServer is the server API for Ipam service.
// All implementations must embed UnimplementedIpamServer
// for forward compatibility
type IpamServer interface {
	MapEndpointIP(context.Context, *MapRequest) (*MapResponse, error)
	UnmapEndpointIP(context.Context, *UnmapRequest) (*UnmapResponse, error)
	GetHomePodIP(context.Context, *GetHomePodIPRequest) (*GetHomePodIPResponse, error)
	BelongsToPodCIDR(context.Context, *BelongsRequest) (*BelongsResponse, error)
	GetClusterIdentity(context.Context, *ClusterIdentityRequest) (*ClusterIdentityResponse, error)
	DoesClusterMappingExist(context.Context, *ClusterMappingRequest) (*ClusterMappingResponse, error)
	mustEmbedUnimplementedIpamServer()
}

// UnimplementedIpamServer must be embedded to have forward compatible implementations.
type UnimplementedIpamServer struct {
}

func (UnimplementedIpamServer) MapEndpointIP(context.Context, *MapRequest) (*MapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MapEndpointIP not implemented")
}
func (UnimplementedIpamServer) UnmapEndpointIP(context.Context, *UnmapRequest) (*UnmapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnmapEndpointIP not implemented")
}
func (UnimplementedIpamServer) GetHomePodIP(context.Context, *GetHomePodIPRequest) (*GetHomePodIPResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetHomePodIP not implemented")
}
func (UnimplementedIpamServer) BelongsToPodCIDR(context.Context, *BelongsRequest) (*BelongsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BelongsToPodCIDR not implemented")
}
func (UnimplementedIpamServer) GetClusterIdentity(context.Context, *ClusterIdentityRequest) (*ClusterIdentityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClusterIdentity not implemented")
}
func (UnimplementedIpamServer) DoesClusterMappingExist(context.Context, *ClusterMappingRequest) (*ClusterMappingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DoesClusterMappingExist not implemented")
}
func (UnimplementedIpamServer) mustEmbedUnimplementedIpamServer() {}

// UnsafeIpamServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IpamServer will
// result in compilation errors.
type UnsafeIpamServer interface {
	mustEmbedUnimplementedIpamServer()
}

func RegisterIpamServer(s grpc.ServiceRegistrar, srv IpamServer) {
	s.RegisterService(&Ipam_ServiceDesc, srv)
}

func _Ipam_MapEndpointIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).MapEndpointIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/MapEndpointIP",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).MapEndpointIP(ctx, req.(*MapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_UnmapEndpointIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UnmapRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).UnmapEndpointIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/UnmapEndpointIP",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).UnmapEndpointIP(ctx, req.(*UnmapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_GetHomePodIP_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetHomePodIPRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).GetHomePodIP(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/GetHomePodIP",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).GetHomePodIP(ctx, req.(*GetHomePodIPRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_BelongsToPodCIDR_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BelongsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).BelongsToPodCIDR(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/BelongsToPodCIDR",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).BelongsToPodCIDR(ctx, req.(*BelongsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_GetClusterIdentity_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterIdentityRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).GetClusterIdentity(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/GetClusterIdentity",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).GetClusterIdentity(ctx, req.(*ClusterIdentityRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_DoesClusterMappingExist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClusterMappingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).DoesClusterMappingExist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ipam/DoesClusterMappingExist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).DoesClusterMappingExist(ctx, req.(*ClusterMappingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Ipam_ServiceDesc is the grpc.ServiceDesc for Ipam service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Ipam_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "ipam",
	HandlerType: (*IpamServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "MapEndpointIP",
			Handler:    _Ipam_MapEndpointIP_Handler,
		},
		{
			MethodName: "UnmapEndpointIP",
			Handler:    _Ipam_UnmapEndpointIP_Handler,
		},
		{
			MethodName: "GetHomePodIP",
			Handler:    _Ipam_GetHomePodIP_Handler,
		},
		{
			MethodName: "BelongsToPodCIDR",
			Handler:    _Ipam_BelongsToPodCIDR_Handler,
		},
		{
			MethodName: "GetClusterIdentity",
			Handler:    _Ipam_GetClusterIdentity_Handler,
		},
		{
			MethodName: "DoesClusterMappingExist",
			Handler:    _Ipam_DoesClusterMappingExist_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/liqonet/ipam/ipam.proto",
}
