// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.17.3
// source: session/session.proto

package session

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// TODO rename to ProtocolId (represents the protocol id)
type ChannelType int32

const (
	ChannelTypeInvalid  ChannelType = 0
	ChannelTypeTransfer ChannelType = 1 // transfer data (files, images, text, ...) between devices
)

// Enum value maps for ChannelType.
var (
	ChannelType_name = map[int32]string{
		0: "CHANNEL_TYPE_INVALID",
		1: "CHANNEL_TYPE_TRANSFER",
	}
	ChannelType_value = map[string]int32{
		"CHANNEL_TYPE_INVALID":  0,
		"CHANNEL_TYPE_TRANSFER": 1,
	}
)

func (x ChannelType) Enum() *ChannelType {
	p := new(ChannelType)
	*p = x
	return p
}

func (x ChannelType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ChannelType) Descriptor() protoreflect.EnumDescriptor {
	return file_session_session_proto_enumTypes[0].Descriptor()
}

func (ChannelType) Type() protoreflect.EnumType {
	return &file_session_session_proto_enumTypes[0]
}

func (x ChannelType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ChannelType.Descriptor instead.
func (ChannelType) EnumDescriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{0}
}

type Open struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// opens a channel with a specific ID.
	// the ID must not be used by any other open channel.
	// the ID should be generated by incrementing the last opened channel ID.
	// the initiator should start at 1 and increment by 2 each time (odd).
	// the responder should start at 2 and increment by 2 each time (even).
	// this way initiator and responder will never collide.
	// the creator of the channel can also be derived from its ID.
	// channel ID 0 is reserved for future use.
	ChannelId uint64 `protobuf:"varint,1,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
	// the channel type determines the protocol that is spoken over this channel.
	// each protocol gets its own protobuf package and set of messages.
	// each protocol must define message types in an enumeration.
	ChannelType ChannelType `protobuf:"varint,2,opt,name=channel_type,json=channelType,proto3,enum=protocol.ChannelType" json:"channel_type,omitempty"`
}

func (x *Open) Reset() {
	*x = Open{}
	if protoimpl.UnsafeEnabled {
		mi := &file_session_session_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Open) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Open) ProtoMessage() {}

func (x *Open) ProtoReflect() protoreflect.Message {
	mi := &file_session_session_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Open.ProtoReflect.Descriptor instead.
func (*Open) Descriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{0}
}

func (x *Open) GetChannelId() uint64 {
	if x != nil {
		return x.ChannelId
	}
	return 0
}

func (x *Open) GetChannelType() ChannelType {
	if x != nil {
		return x.ChannelType
	}
	return ChannelTypeInvalid
}

type Accept struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// the channel has been accepted by the peer
	// who is now listening for messages.
	// only now is it guaranteed that messages on this channel will reach the other device.
	ChannelId uint64 `protobuf:"varint,1,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
}

func (x *Accept) Reset() {
	*x = Accept{}
	if protoimpl.UnsafeEnabled {
		mi := &file_session_session_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Accept) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Accept) ProtoMessage() {}

func (x *Accept) ProtoReflect() protoreflect.Message {
	mi := &file_session_session_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Accept.ProtoReflect.Descriptor instead.
func (*Accept) Descriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{1}
}

func (x *Accept) GetChannelId() uint64 {
	if x != nil {
		return x.ChannelId
	}
	return 0
}

type Close struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// closes a channel with a specific ID.
	// messages on this channel are discarded after it is closed.
	// a device should be careful with closing channels of the other peer.
	// usually the creator of the channel should be the one who closes it.
	ChannelId uint64 `protobuf:"varint,1,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
}

func (x *Close) Reset() {
	*x = Close{}
	if protoimpl.UnsafeEnabled {
		mi := &file_session_session_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Close) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Close) ProtoMessage() {}

func (x *Close) ProtoReflect() protoreflect.Message {
	mi := &file_session_session_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Close.ProtoReflect.Descriptor instead.
func (*Close) Descriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{2}
}

func (x *Close) GetChannelId() uint64 {
	if x != nil {
		return x.ChannelId
	}
	return 0
}

type Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// contains data that is to be transmitted on a specific channel.
	// will be discarded by the receiver if that channel has been closed.
	ChannelId uint64 `protobuf:"varint,1,opt,name=channel_id,json=channelId,proto3" json:"channel_id,omitempty"`
	// Holds the value of the message type of the respective protocol (depends on channel_type).
	// It is an integer instead of a specific enum, since protocols use different enums.
	MessageType int32 `protobuf:"varint,2,opt,name=message_type,json=messageType,proto3" json:"message_type,omitempty"`
	// the payload contains the encoded protobuf message of this channel's protocol.
	Payload []byte `protobuf:"bytes,3,opt,name=payload,proto3" json:"payload,omitempty"`
}

func (x *Data) Reset() {
	*x = Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_session_session_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Data) ProtoMessage() {}

func (x *Data) ProtoReflect() protoreflect.Message {
	mi := &file_session_session_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Data.ProtoReflect.Descriptor instead.
func (*Data) Descriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{3}
}

func (x *Data) GetChannelId() uint64 {
	if x != nil {
		return x.ChannelId
	}
	return 0
}

func (x *Data) GetMessageType() int32 {
	if x != nil {
		return x.MessageType
	}
	return 0
}

func (x *Data) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

type Message struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// using oneof here spares us from using another message_id field here
	// and specifying the message type in the program code
	// which could be another source of potential bugs.
	//
	// Types that are assignable to Content:
	//	*Message_Open
	//	*Message_Accept
	//	*Message_Close
	//	*Message_Data
	Content isMessage_Content `protobuf_oneof:"content"`
}

func (x *Message) Reset() {
	*x = Message{}
	if protoimpl.UnsafeEnabled {
		mi := &file_session_session_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	mi := &file_session_session_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Message.ProtoReflect.Descriptor instead.
func (*Message) Descriptor() ([]byte, []int) {
	return file_session_session_proto_rawDescGZIP(), []int{4}
}

func (m *Message) GetContent() isMessage_Content {
	if m != nil {
		return m.Content
	}
	return nil
}

func (x *Message) GetOpen() *Open {
	if x, ok := x.GetContent().(*Message_Open); ok {
		return x.Open
	}
	return nil
}

func (x *Message) GetAccept() *Accept {
	if x, ok := x.GetContent().(*Message_Accept); ok {
		return x.Accept
	}
	return nil
}

func (x *Message) GetClose() *Close {
	if x, ok := x.GetContent().(*Message_Close); ok {
		return x.Close
	}
	return nil
}

func (x *Message) GetData() *Data {
	if x, ok := x.GetContent().(*Message_Data); ok {
		return x.Data
	}
	return nil
}

type isMessage_Content interface {
	isMessage_Content()
}

type Message_Open struct {
	Open *Open `protobuf:"bytes,1,opt,name=open,proto3,oneof"`
}

type Message_Accept struct {
	Accept *Accept `protobuf:"bytes,2,opt,name=accept,proto3,oneof"`
}

type Message_Close struct {
	Close *Close `protobuf:"bytes,3,opt,name=close,proto3,oneof"`
}

type Message_Data struct {
	Data *Data `protobuf:"bytes,4,opt,name=data,proto3,oneof"`
}

func (*Message_Open) isMessage_Content() {}

func (*Message_Accept) isMessage_Content() {}

func (*Message_Close) isMessage_Content() {}

func (*Message_Data) isMessage_Content() {}

var File_session_session_proto protoreflect.FileDescriptor

var file_session_session_proto_rawDesc = []byte{
	0x0a, 0x15, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6c, 0x22, 0x5f, 0x0a, 0x04, 0x4f, 0x70, 0x65, 0x6e, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x68, 0x61,
	0x6e, 0x6e, 0x65, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x63,
	0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x64, 0x12, 0x38, 0x0a, 0x0c, 0x63, 0x68, 0x61, 0x6e,
	0x6e, 0x65, 0x6c, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x15,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x43, 0x68, 0x61, 0x6e, 0x6e, 0x65,
	0x6c, 0x54, 0x79, 0x70, 0x65, 0x52, 0x0b, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x54, 0x79,
	0x70, 0x65, 0x22, 0x27, 0x0a, 0x06, 0x41, 0x63, 0x63, 0x65, 0x70, 0x74, 0x12, 0x1d, 0x0a, 0x0a,
	0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x64, 0x22, 0x26, 0x0a, 0x05, 0x43,
	0x6c, 0x6f, 0x73, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x5f,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65,
	0x6c, 0x49, 0x64, 0x22, 0x62, 0x0a, 0x04, 0x44, 0x61, 0x74, 0x61, 0x12, 0x1d, 0x0a, 0x0a, 0x63,
	0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x09, 0x63, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x49, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x0b, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07,
	0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x22, 0xb5, 0x01, 0x0a, 0x07, 0x4d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x6f, 0x70, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x0e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x4f, 0x70, 0x65,
	0x6e, 0x48, 0x00, 0x52, 0x04, 0x6f, 0x70, 0x65, 0x6e, 0x12, 0x2a, 0x0a, 0x06, 0x61, 0x63, 0x63,
	0x65, 0x70, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x41, 0x63, 0x63, 0x65, 0x70, 0x74, 0x48, 0x00, 0x52, 0x06, 0x61,
	0x63, 0x63, 0x65, 0x70, 0x74, 0x12, 0x27, 0x0a, 0x05, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e,
	0x43, 0x6c, 0x6f, 0x73, 0x65, 0x48, 0x00, 0x52, 0x05, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x12, 0x24,
	0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x42, 0x09, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x2a,
	0x42, 0x0a, 0x0b, 0x43, 0x68, 0x61, 0x6e, 0x6e, 0x65, 0x6c, 0x54, 0x79, 0x70, 0x65, 0x12, 0x18,
	0x0a, 0x14, 0x43, 0x48, 0x41, 0x4e, 0x4e, 0x45, 0x4c, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x49,
	0x4e, 0x56, 0x41, 0x4c, 0x49, 0x44, 0x10, 0x00, 0x12, 0x19, 0x0a, 0x15, 0x43, 0x48, 0x41, 0x4e,
	0x4e, 0x45, 0x4c, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x54, 0x52, 0x41, 0x4e, 0x53, 0x46, 0x45,
	0x52, 0x10, 0x01, 0x42, 0x1a, 0x5a, 0x18, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x6b, 0x74, 0x2f, 0x63,
	0x6f, 0x72, 0x65, 0x2f, 0x6c, 0x69, 0x62, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_session_session_proto_rawDescOnce sync.Once
	file_session_session_proto_rawDescData = file_session_session_proto_rawDesc
)

func file_session_session_proto_rawDescGZIP() []byte {
	file_session_session_proto_rawDescOnce.Do(func() {
		file_session_session_proto_rawDescData = protoimpl.X.CompressGZIP(file_session_session_proto_rawDescData)
	})
	return file_session_session_proto_rawDescData
}

var file_session_session_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_session_session_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_session_session_proto_goTypes = []interface{}{
	(ChannelType)(0), // 0: protocol.ChannelType
	(*Open)(nil),     // 1: protocol.Open
	(*Accept)(nil),   // 2: protocol.Accept
	(*Close)(nil),    // 3: protocol.Close
	(*Data)(nil),     // 4: protocol.Data
	(*Message)(nil),  // 5: protocol.Message
}
var file_session_session_proto_depIdxs = []int32{
	0, // 0: protocol.Open.channel_type:type_name -> protocol.ChannelType
	1, // 1: protocol.Message.open:type_name -> protocol.Open
	2, // 2: protocol.Message.accept:type_name -> protocol.Accept
	3, // 3: protocol.Message.close:type_name -> protocol.Close
	4, // 4: protocol.Message.data:type_name -> protocol.Data
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_session_session_proto_init() }
func file_session_session_proto_init() {
	if File_session_session_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_session_session_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Open); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_session_session_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Accept); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_session_session_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Close); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_session_session_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Data); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_session_session_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Message); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_session_session_proto_msgTypes[4].OneofWrappers = []interface{}{
		(*Message_Open)(nil),
		(*Message_Accept)(nil),
		(*Message_Close)(nil),
		(*Message_Data)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_session_session_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_session_session_proto_goTypes,
		DependencyIndexes: file_session_session_proto_depIdxs,
		EnumInfos:         file_session_session_proto_enumTypes,
		MessageInfos:      file_session_session_proto_msgTypes,
	}.Build()
	File_session_session_proto = out.File
	file_session_session_proto_rawDesc = nil
	file_session_session_proto_goTypes = nil
	file_session_session_proto_depIdxs = nil
}