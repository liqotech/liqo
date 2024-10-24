// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.28.3
// source: pkg/ipamold/ipam.proto

package ipam

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Ipam_MapEndpointIP_FullMethodName        = "/ipam/MapEndpointIP"
	Ipam_UnmapEndpointIP_FullMethodName      = "/ipam/UnmapEndpointIP"
	Ipam_MapNetworkCIDR_FullMethodName       = "/ipam/MapNetworkCIDR"
	Ipam_UnmapNetworkCIDR_FullMethodName     = "/ipam/UnmapNetworkCIDR"
	Ipam_GetHomePodIP_FullMethodName         = "/ipam/GetHomePodIP"
	Ipam_BelongsToPodCIDR_FullMethodName     = "/ipam/BelongsToPodCIDR"
	Ipam_GetOrSetExternalCIDR_FullMethodName = "/ipam/GetOrSetExternalCIDR"
	Ipam_SetSubnetsPerCluster_FullMethodName = "/ipam/SetSubnetsPerCluster"
)

// IpamClient is the client API for Ipam service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IpamClient interface {
	MapEndpointIP(ctx context.Context, in *MapRequest, opts ...grpc.CallOption) (*MapResponse, error)
	UnmapEndpointIP(ctx context.Context, in *UnmapRequest, opts ...grpc.CallOption) (*UnmapResponse, error)
	MapNetworkCIDR(ctx context.Context, in *MapCIDRRequest, opts ...grpc.CallOption) (*MapCIDRResponse, error)
	UnmapNetworkCIDR(ctx context.Context, in *UnmapCIDRRequest, opts ...grpc.CallOption) (*UnmapCIDRResponse, error)
	GetHomePodIP(ctx context.Context, in *GetHomePodIPRequest, opts ...grpc.CallOption) (*GetHomePodIPResponse, error)
	BelongsToPodCIDR(ctx context.Context, in *BelongsRequest, opts ...grpc.CallOption) (*BelongsResponse, error)
	GetOrSetExternalCIDR(ctx context.Context, in *GetOrSetExtCIDRRequest, opts ...grpc.CallOption) (*GetOrSetExtCIDRResponse, error)
	SetSubnetsPerCluster(ctx context.Context, in *SetSubnetsPerClusterRequest, opts ...grpc.CallOption) (*SetSubnetsPerClusterResponse, error)
}

type ipamClient struct {
	cc grpc.ClientConnInterface
}

func NewIpamClient(cc grpc.ClientConnInterface) IpamClient {
	return &ipamClient{cc}
}

func (c *ipamClient) MapEndpointIP(ctx context.Context, in *MapRequest, opts ...grpc.CallOption) (*MapResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(MapResponse)
	err := c.cc.Invoke(ctx, Ipam_MapEndpointIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) UnmapEndpointIP(ctx context.Context, in *UnmapRequest, opts ...grpc.CallOption) (*UnmapResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UnmapResponse)
	err := c.cc.Invoke(ctx, Ipam_UnmapEndpointIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) MapNetworkCIDR(ctx context.Context, in *MapCIDRRequest, opts ...grpc.CallOption) (*MapCIDRResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(MapCIDRResponse)
	err := c.cc.Invoke(ctx, Ipam_MapNetworkCIDR_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) UnmapNetworkCIDR(ctx context.Context, in *UnmapCIDRRequest, opts ...grpc.CallOption) (*UnmapCIDRResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UnmapCIDRResponse)
	err := c.cc.Invoke(ctx, Ipam_UnmapNetworkCIDR_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) GetHomePodIP(ctx context.Context, in *GetHomePodIPRequest, opts ...grpc.CallOption) (*GetHomePodIPResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetHomePodIPResponse)
	err := c.cc.Invoke(ctx, Ipam_GetHomePodIP_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) BelongsToPodCIDR(ctx context.Context, in *BelongsRequest, opts ...grpc.CallOption) (*BelongsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(BelongsResponse)
	err := c.cc.Invoke(ctx, Ipam_BelongsToPodCIDR_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) GetOrSetExternalCIDR(ctx context.Context, in *GetOrSetExtCIDRRequest, opts ...grpc.CallOption) (*GetOrSetExtCIDRResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetOrSetExtCIDRResponse)
	err := c.cc.Invoke(ctx, Ipam_GetOrSetExternalCIDR_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ipamClient) SetSubnetsPerCluster(ctx context.Context, in *SetSubnetsPerClusterRequest, opts ...grpc.CallOption) (*SetSubnetsPerClusterResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SetSubnetsPerClusterResponse)
	err := c.cc.Invoke(ctx, Ipam_SetSubnetsPerCluster_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// IpamServer is the server API for Ipam service.
// All implementations must embed UnimplementedIpamServer
// for forward compatibility.
type IpamServer interface {
	MapEndpointIP(context.Context, *MapRequest) (*MapResponse, error)
	UnmapEndpointIP(context.Context, *UnmapRequest) (*UnmapResponse, error)
	MapNetworkCIDR(context.Context, *MapCIDRRequest) (*MapCIDRResponse, error)
	UnmapNetworkCIDR(context.Context, *UnmapCIDRRequest) (*UnmapCIDRResponse, error)
	GetHomePodIP(context.Context, *GetHomePodIPRequest) (*GetHomePodIPResponse, error)
	BelongsToPodCIDR(context.Context, *BelongsRequest) (*BelongsResponse, error)
	GetOrSetExternalCIDR(context.Context, *GetOrSetExtCIDRRequest) (*GetOrSetExtCIDRResponse, error)
	SetSubnetsPerCluster(context.Context, *SetSubnetsPerClusterRequest) (*SetSubnetsPerClusterResponse, error)
	mustEmbedUnimplementedIpamServer()
}

// UnimplementedIpamServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedIpamServer struct{}

func (UnimplementedIpamServer) MapEndpointIP(context.Context, *MapRequest) (*MapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MapEndpointIP not implemented")
}
func (UnimplementedIpamServer) UnmapEndpointIP(context.Context, *UnmapRequest) (*UnmapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnmapEndpointIP not implemented")
}
func (UnimplementedIpamServer) MapNetworkCIDR(context.Context, *MapCIDRRequest) (*MapCIDRResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MapNetworkCIDR not implemented")
}
func (UnimplementedIpamServer) UnmapNetworkCIDR(context.Context, *UnmapCIDRRequest) (*UnmapCIDRResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnmapNetworkCIDR not implemented")
}
func (UnimplementedIpamServer) GetHomePodIP(context.Context, *GetHomePodIPRequest) (*GetHomePodIPResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetHomePodIP not implemented")
}
func (UnimplementedIpamServer) BelongsToPodCIDR(context.Context, *BelongsRequest) (*BelongsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BelongsToPodCIDR not implemented")
}
func (UnimplementedIpamServer) GetOrSetExternalCIDR(context.Context, *GetOrSetExtCIDRRequest) (*GetOrSetExtCIDRResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetOrSetExternalCIDR not implemented")
}
func (UnimplementedIpamServer) SetSubnetsPerCluster(context.Context, *SetSubnetsPerClusterRequest) (*SetSubnetsPerClusterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetSubnetsPerCluster not implemented")
}
func (UnimplementedIpamServer) mustEmbedUnimplementedIpamServer() {}
func (UnimplementedIpamServer) testEmbeddedByValue()              {}

// UnsafeIpamServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IpamServer will
// result in compilation errors.
type UnsafeIpamServer interface {
	mustEmbedUnimplementedIpamServer()
}

func RegisterIpamServer(s grpc.ServiceRegistrar, srv IpamServer) {
	// If the following call pancis, it indicates UnimplementedIpamServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
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
		FullMethod: Ipam_MapEndpointIP_FullMethodName,
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
		FullMethod: Ipam_UnmapEndpointIP_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).UnmapEndpointIP(ctx, req.(*UnmapRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_MapNetworkCIDR_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MapCIDRRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).MapNetworkCIDR(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Ipam_MapNetworkCIDR_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).MapNetworkCIDR(ctx, req.(*MapCIDRRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_UnmapNetworkCIDR_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UnmapCIDRRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).UnmapNetworkCIDR(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Ipam_UnmapNetworkCIDR_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).UnmapNetworkCIDR(ctx, req.(*UnmapCIDRRequest))
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
		FullMethod: Ipam_GetHomePodIP_FullMethodName,
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
		FullMethod: Ipam_BelongsToPodCIDR_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).BelongsToPodCIDR(ctx, req.(*BelongsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_GetOrSetExternalCIDR_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetOrSetExtCIDRRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).GetOrSetExternalCIDR(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Ipam_GetOrSetExternalCIDR_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).GetOrSetExternalCIDR(ctx, req.(*GetOrSetExtCIDRRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ipam_SetSubnetsPerCluster_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetSubnetsPerClusterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IpamServer).SetSubnetsPerCluster(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Ipam_SetSubnetsPerCluster_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IpamServer).SetSubnetsPerCluster(ctx, req.(*SetSubnetsPerClusterRequest))
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
			MethodName: "MapNetworkCIDR",
			Handler:    _Ipam_MapNetworkCIDR_Handler,
		},
		{
			MethodName: "UnmapNetworkCIDR",
			Handler:    _Ipam_UnmapNetworkCIDR_Handler,
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
			MethodName: "GetOrSetExternalCIDR",
			Handler:    _Ipam_GetOrSetExternalCIDR_Handler,
		},
		{
			MethodName: "SetSubnetsPerCluster",
			Handler:    _Ipam_SetSubnetsPerCluster_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/ipamold/ipam.proto",
}
