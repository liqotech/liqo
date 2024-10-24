// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.1
// 	protoc        v5.28.3
// source: pkg/ipam/ipam.proto

package ipam

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ResponseResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Success bool   `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
	Error   string `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *ResponseResult) Reset() {
	*x = ResponseResult{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ResponseResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseResult) ProtoMessage() {}

func (x *ResponseResult) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseResult.ProtoReflect.Descriptor instead.
func (*ResponseResult) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{0}
}

func (x *ResponseResult) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

func (x *ResponseResult) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type IPAcquireRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ip   string `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	Cidr string `protobuf:"bytes,2,opt,name=cidr,proto3" json:"cidr,omitempty"`
}

func (x *IPAcquireRequest) Reset() {
	*x = IPAcquireRequest{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPAcquireRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPAcquireRequest) ProtoMessage() {}

func (x *IPAcquireRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPAcquireRequest.ProtoReflect.Descriptor instead.
func (*IPAcquireRequest) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{1}
}

func (x *IPAcquireRequest) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

func (x *IPAcquireRequest) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

type IPAcquireResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result *ResponseResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
	Ip     string          `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
}

func (x *IPAcquireResponse) Reset() {
	*x = IPAcquireResponse{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPAcquireResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPAcquireResponse) ProtoMessage() {}

func (x *IPAcquireResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPAcquireResponse.ProtoReflect.Descriptor instead.
func (*IPAcquireResponse) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{2}
}

func (x *IPAcquireResponse) GetResult() *ResponseResult {
	if x != nil {
		return x.Result
	}
	return nil
}

func (x *IPAcquireResponse) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

type IPReleaseRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ip   string `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	Cidr string `protobuf:"bytes,2,opt,name=cidr,proto3" json:"cidr,omitempty"`
}

func (x *IPReleaseRequest) Reset() {
	*x = IPReleaseRequest{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPReleaseRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPReleaseRequest) ProtoMessage() {}

func (x *IPReleaseRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPReleaseRequest.ProtoReflect.Descriptor instead.
func (*IPReleaseRequest) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{3}
}

func (x *IPReleaseRequest) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

func (x *IPReleaseRequest) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

type IPReleaseResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result *ResponseResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *IPReleaseResponse) Reset() {
	*x = IPReleaseResponse{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *IPReleaseResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPReleaseResponse) ProtoMessage() {}

func (x *IPReleaseResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPReleaseResponse.ProtoReflect.Descriptor instead.
func (*IPReleaseResponse) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{4}
}

func (x *IPReleaseResponse) GetResult() *ResponseResult {
	if x != nil {
		return x.Result
	}
	return nil
}

type NetworkAcquireRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cidr      string `protobuf:"bytes,1,opt,name=cidr,proto3" json:"cidr,omitempty"`
	Immutable bool   `protobuf:"varint,2,opt,name=immutable,proto3" json:"immutable,omitempty"` // If true, the network cannot be remapped. It will be allocated if available, or an error will be returned.
}

func (x *NetworkAcquireRequest) Reset() {
	*x = NetworkAcquireRequest{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkAcquireRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkAcquireRequest) ProtoMessage() {}

func (x *NetworkAcquireRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkAcquireRequest.ProtoReflect.Descriptor instead.
func (*NetworkAcquireRequest) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{5}
}

func (x *NetworkAcquireRequest) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

func (x *NetworkAcquireRequest) GetImmutable() bool {
	if x != nil {
		return x.Immutable
	}
	return false
}

type NetworkAcquireResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result *ResponseResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
	Cidr   string          `protobuf:"bytes,2,opt,name=cidr,proto3" json:"cidr,omitempty"`
}

func (x *NetworkAcquireResponse) Reset() {
	*x = NetworkAcquireResponse{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkAcquireResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkAcquireResponse) ProtoMessage() {}

func (x *NetworkAcquireResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkAcquireResponse.ProtoReflect.Descriptor instead.
func (*NetworkAcquireResponse) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{6}
}

func (x *NetworkAcquireResponse) GetResult() *ResponseResult {
	if x != nil {
		return x.Result
	}
	return nil
}

func (x *NetworkAcquireResponse) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

type NetworkReleaseRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cidr string `protobuf:"bytes,1,opt,name=cidr,proto3" json:"cidr,omitempty"`
}

func (x *NetworkReleaseRequest) Reset() {
	*x = NetworkReleaseRequest{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkReleaseRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkReleaseRequest) ProtoMessage() {}

func (x *NetworkReleaseRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkReleaseRequest.ProtoReflect.Descriptor instead.
func (*NetworkReleaseRequest) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{7}
}

func (x *NetworkReleaseRequest) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

type NetworkReleaseResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result *ResponseResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *NetworkReleaseResponse) Reset() {
	*x = NetworkReleaseResponse{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkReleaseResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkReleaseResponse) ProtoMessage() {}

func (x *NetworkReleaseResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkReleaseResponse.ProtoReflect.Descriptor instead.
func (*NetworkReleaseResponse) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{8}
}

func (x *NetworkReleaseResponse) GetResult() *ResponseResult {
	if x != nil {
		return x.Result
	}
	return nil
}

type NetworkAvailableRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cidr string `protobuf:"bytes,1,opt,name=cidr,proto3" json:"cidr,omitempty"`
}

func (x *NetworkAvailableRequest) Reset() {
	*x = NetworkAvailableRequest{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkAvailableRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkAvailableRequest) ProtoMessage() {}

func (x *NetworkAvailableRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkAvailableRequest.ProtoReflect.Descriptor instead.
func (*NetworkAvailableRequest) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{9}
}

func (x *NetworkAvailableRequest) GetCidr() string {
	if x != nil {
		return x.Cidr
	}
	return ""
}

type NetworkAvailableResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result    *ResponseResult `protobuf:"bytes,1,opt,name=result,proto3" json:"result,omitempty"`
	Available bool            `protobuf:"varint,2,opt,name=available,proto3" json:"available,omitempty"`
}

func (x *NetworkAvailableResponse) Reset() {
	*x = NetworkAvailableResponse{}
	mi := &file_pkg_ipam_ipam_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *NetworkAvailableResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkAvailableResponse) ProtoMessage() {}

func (x *NetworkAvailableResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_ipam_ipam_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkAvailableResponse.ProtoReflect.Descriptor instead.
func (*NetworkAvailableResponse) Descriptor() ([]byte, []int) {
	return file_pkg_ipam_ipam_proto_rawDescGZIP(), []int{10}
}

func (x *NetworkAvailableResponse) GetResult() *ResponseResult {
	if x != nil {
		return x.Result
	}
	return nil
}

func (x *NetworkAvailableResponse) GetAvailable() bool {
	if x != nil {
		return x.Available
	}
	return false
}

var File_pkg_ipam_ipam_proto protoreflect.FileDescriptor

var file_pkg_ipam_ipam_proto_rawDesc = []byte{
	0x0a, 0x13, 0x70, 0x6b, 0x67, 0x2f, 0x69, 0x70, 0x61, 0x6d, 0x2f, 0x69, 0x70, 0x61, 0x6d, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x40, 0x0a, 0x0e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73,
	0x73, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x22, 0x36, 0x0a, 0x10, 0x49, 0x50, 0x41, 0x63, 0x71,
	0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x63,
	0x69, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x69, 0x64, 0x72, 0x22,
	0x4c, 0x0a, 0x11, 0x49, 0x50, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x27, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x70, 0x22, 0x36, 0x0a,
	0x10, 0x49, 0x50, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x70, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x69, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x63, 0x69, 0x64, 0x72, 0x22, 0x3c, 0x0a, 0x11, 0x49, 0x50, 0x52, 0x65, 0x6c, 0x65, 0x61,
	0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x27, 0x0a, 0x06, 0x72, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72, 0x65, 0x73,
	0x75, 0x6c, 0x74, 0x22, 0x49, 0x0a, 0x15, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41, 0x63,
	0x71, 0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04,
	0x63, 0x69, 0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x69, 0x64, 0x72,
	0x12, 0x1c, 0x0a, 0x09, 0x69, 0x6d, 0x6d, 0x75, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x09, 0x69, 0x6d, 0x6d, 0x75, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x22, 0x55,
	0x0a, 0x16, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x27, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75,
	0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x69, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x63, 0x69, 0x64, 0x72, 0x22, 0x2b, 0x0a, 0x15, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b,
	0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12,
	0x0a, 0x04, 0x63, 0x69, 0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x69,
	0x64, 0x72, 0x22, 0x41, 0x0a, 0x16, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x52, 0x65, 0x6c,
	0x65, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x27, 0x0a, 0x06,
	0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52, 0x06, 0x72,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x22, 0x2d, 0x0a, 0x17, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b,
	0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x63, 0x69, 0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x63, 0x69, 0x64, 0x72, 0x22, 0x61, 0x0a, 0x18, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41,
	0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x27, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x0f, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x61, 0x76, 0x61,
	0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x61, 0x76,
	0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x32, 0xbf, 0x02, 0x0a, 0x04, 0x49, 0x50, 0x41, 0x4d,
	0x12, 0x32, 0x0a, 0x09, 0x49, 0x50, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x12, 0x11, 0x2e,
	0x49, 0x50, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x12, 0x2e, 0x49, 0x50, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x32, 0x0a, 0x09, 0x49, 0x50, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73,
	0x65, 0x12, 0x11, 0x2e, 0x49, 0x50, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x12, 0x2e, 0x49, 0x50, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x41, 0x0a, 0x0e, 0x4e, 0x65, 0x74, 0x77,
	0x6f, 0x72, 0x6b, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x12, 0x16, 0x2e, 0x4e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x41, 0x63, 0x71, 0x75, 0x69, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x17, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41, 0x63, 0x71, 0x75,
	0x69, 0x72, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x41, 0x0a, 0x0e, 0x4e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x12, 0x16, 0x2e,
	0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x52,
	0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x49,
	0x0a, 0x12, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x49, 0x73, 0x41, 0x76, 0x61, 0x69, 0x6c,
	0x61, 0x62, 0x6c, 0x65, 0x12, 0x18, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41, 0x76,
	0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19,
	0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c,
	0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x08, 0x5a, 0x06, 0x2e, 0x2f, 0x69,
	0x70, 0x61, 0x6d, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_ipam_ipam_proto_rawDescOnce sync.Once
	file_pkg_ipam_ipam_proto_rawDescData = file_pkg_ipam_ipam_proto_rawDesc
)

func file_pkg_ipam_ipam_proto_rawDescGZIP() []byte {
	file_pkg_ipam_ipam_proto_rawDescOnce.Do(func() {
		file_pkg_ipam_ipam_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_ipam_ipam_proto_rawDescData)
	})
	return file_pkg_ipam_ipam_proto_rawDescData
}

var file_pkg_ipam_ipam_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_pkg_ipam_ipam_proto_goTypes = []any{
	(*ResponseResult)(nil),           // 0: ResponseResult
	(*IPAcquireRequest)(nil),         // 1: IPAcquireRequest
	(*IPAcquireResponse)(nil),        // 2: IPAcquireResponse
	(*IPReleaseRequest)(nil),         // 3: IPReleaseRequest
	(*IPReleaseResponse)(nil),        // 4: IPReleaseResponse
	(*NetworkAcquireRequest)(nil),    // 5: NetworkAcquireRequest
	(*NetworkAcquireResponse)(nil),   // 6: NetworkAcquireResponse
	(*NetworkReleaseRequest)(nil),    // 7: NetworkReleaseRequest
	(*NetworkReleaseResponse)(nil),   // 8: NetworkReleaseResponse
	(*NetworkAvailableRequest)(nil),  // 9: NetworkAvailableRequest
	(*NetworkAvailableResponse)(nil), // 10: NetworkAvailableResponse
}
var file_pkg_ipam_ipam_proto_depIdxs = []int32{
	0,  // 0: IPAcquireResponse.result:type_name -> ResponseResult
	0,  // 1: IPReleaseResponse.result:type_name -> ResponseResult
	0,  // 2: NetworkAcquireResponse.result:type_name -> ResponseResult
	0,  // 3: NetworkReleaseResponse.result:type_name -> ResponseResult
	0,  // 4: NetworkAvailableResponse.result:type_name -> ResponseResult
	1,  // 5: IPAM.IPAcquire:input_type -> IPAcquireRequest
	3,  // 6: IPAM.IPRelease:input_type -> IPReleaseRequest
	5,  // 7: IPAM.NetworkAcquire:input_type -> NetworkAcquireRequest
	7,  // 8: IPAM.NetworkRelease:input_type -> NetworkReleaseRequest
	9,  // 9: IPAM.NetworkIsAvailable:input_type -> NetworkAvailableRequest
	2,  // 10: IPAM.IPAcquire:output_type -> IPAcquireResponse
	4,  // 11: IPAM.IPRelease:output_type -> IPReleaseResponse
	6,  // 12: IPAM.NetworkAcquire:output_type -> NetworkAcquireResponse
	8,  // 13: IPAM.NetworkRelease:output_type -> NetworkReleaseResponse
	10, // 14: IPAM.NetworkIsAvailable:output_type -> NetworkAvailableResponse
	10, // [10:15] is the sub-list for method output_type
	5,  // [5:10] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_pkg_ipam_ipam_proto_init() }
func file_pkg_ipam_ipam_proto_init() {
	if File_pkg_ipam_ipam_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_ipam_ipam_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_ipam_ipam_proto_goTypes,
		DependencyIndexes: file_pkg_ipam_ipam_proto_depIdxs,
		MessageInfos:      file_pkg_ipam_ipam_proto_msgTypes,
	}.Build()
	File_pkg_ipam_ipam_proto = out.File
	file_pkg_ipam_ipam_proto_rawDesc = nil
	file_pkg_ipam_ipam_proto_goTypes = nil
	file_pkg_ipam_ipam_proto_depIdxs = nil
}
