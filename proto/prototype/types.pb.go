// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.21.6
// source: prototype/types.proto

package prototype

import (
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	_ "github.com/raidoNetwork/RDO_v2/proto/ext"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/descriptorpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Block struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Num          uint64         `protobuf:"varint,1,opt,name=num,proto3" json:"num,omitempty"`
	Slot         uint64         `protobuf:"varint,2,opt,name=slot,proto3" json:"slot,omitempty"`
	Version      []byte         `protobuf:"bytes,3,opt,name=version,proto3" json:"version,omitempty" ssz-size:"3"`
	Hash         []byte         `protobuf:"bytes,4,opt,name=hash,proto3" json:"hash,omitempty" ssz-size:"32"`
	Parent       []byte         `protobuf:"bytes,5,opt,name=parent,proto3" json:"parent,omitempty" ssz-size:"32"`
	Timestamp    uint64         `protobuf:"varint,6,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Txroot       []byte         `protobuf:"bytes,7,opt,name=txroot,proto3" json:"txroot,omitempty" ssz-size:"32"`
	Proposer     *Sign          `protobuf:"bytes,8,opt,name=proposer,proto3" json:"proposer,omitempty"`
	Approvers    []*Sign        `protobuf:"bytes,9,rep,name=approvers,proto3" json:"approvers,omitempty" ssz-max:"128"`
	Slashers     []*Sign        `protobuf:"bytes,10,rep,name=slashers,proto3" json:"slashers,omitempty" ssz-max:"128"`
	Transactions []*Transaction `protobuf:"bytes,11,rep,name=transactions,proto3" json:"transactions,omitempty" ssz-max:"1500"`
}

func (x *Block) Reset() {
	*x = Block{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Block) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Block) ProtoMessage() {}

func (x *Block) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Block.ProtoReflect.Descriptor instead.
func (*Block) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{0}
}

func (x *Block) GetNum() uint64 {
	if x != nil {
		return x.Num
	}
	return 0
}

func (x *Block) GetSlot() uint64 {
	if x != nil {
		return x.Slot
	}
	return 0
}

func (x *Block) GetVersion() []byte {
	if x != nil {
		return x.Version
	}
	return nil
}

func (x *Block) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Block) GetParent() []byte {
	if x != nil {
		return x.Parent
	}
	return nil
}

func (x *Block) GetTimestamp() uint64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Block) GetTxroot() []byte {
	if x != nil {
		return x.Txroot
	}
	return nil
}

func (x *Block) GetProposer() *Sign {
	if x != nil {
		return x.Proposer
	}
	return nil
}

func (x *Block) GetApprovers() []*Sign {
	if x != nil {
		return x.Approvers
	}
	return nil
}

func (x *Block) GetSlashers() []*Sign {
	if x != nil {
		return x.Slashers
	}
	return nil
}

func (x *Block) GetTransactions() []*Transaction {
	if x != nil {
		return x.Transactions
	}
	return nil
}

type Sign struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address   []byte `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty" ssz-size:"20"`
	Signature []byte `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty" ssz-size:"65"`
}

func (x *Sign) Reset() {
	*x = Sign{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Sign) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Sign) ProtoMessage() {}

func (x *Sign) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Sign.ProtoReflect.Descriptor instead.
func (*Sign) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{1}
}

func (x *Sign) GetAddress() []byte {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *Sign) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

type Transaction struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Num       uint64      `protobuf:"varint,1,opt,name=num,proto3" json:"num,omitempty"`
	Type      uint32      `protobuf:"varint,2,opt,name=type,proto3" json:"type,omitempty"`
	Timestamp uint64      `protobuf:"varint,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Hash      []byte      `protobuf:"bytes,4,opt,name=hash,proto3" json:"hash,omitempty" ssz-size:"32"`
	Fee       uint64      `protobuf:"varint,5,opt,name=fee,proto3" json:"fee,omitempty"`
	Data      []byte      `protobuf:"bytes,6,opt,name=data,proto3" json:"data,omitempty" ssz-max:"1000000"` // external byte data
	Inputs    []*TxInput  `protobuf:"bytes,7,rep,name=inputs,proto3" json:"inputs,omitempty" ssz-max:"2000"`
	Outputs   []*TxOutput `protobuf:"bytes,8,rep,name=outputs,proto3" json:"outputs,omitempty" ssz-max:"2000"`
	Signature []byte      `protobuf:"bytes,9,opt,name=signature,proto3" json:"signature,omitempty" ssz-size:"65"`
	Status    uint32      `protobuf:"varint,10,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *Transaction) Reset() {
	*x = Transaction{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Transaction) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Transaction) ProtoMessage() {}

func (x *Transaction) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Transaction.ProtoReflect.Descriptor instead.
func (*Transaction) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{2}
}

func (x *Transaction) GetNum() uint64 {
	if x != nil {
		return x.Num
	}
	return 0
}

func (x *Transaction) GetType() uint32 {
	if x != nil {
		return x.Type
	}
	return 0
}

func (x *Transaction) GetTimestamp() uint64 {
	if x != nil {
		return x.Timestamp
	}
	return 0
}

func (x *Transaction) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *Transaction) GetFee() uint64 {
	if x != nil {
		return x.Fee
	}
	return 0
}

func (x *Transaction) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Transaction) GetInputs() []*TxInput {
	if x != nil {
		return x.Inputs
	}
	return nil
}

func (x *Transaction) GetOutputs() []*TxOutput {
	if x != nil {
		return x.Outputs
	}
	return nil
}

func (x *Transaction) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

func (x *Transaction) GetStatus() uint32 {
	if x != nil {
		return x.Status
	}
	return 0
}

type TxInput struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash    []byte `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty" ssz-size:"32"`
	Index   uint32 `protobuf:"varint,2,opt,name=index,proto3" json:"index,omitempty"`
	Address []byte `protobuf:"bytes,3,opt,name=address,proto3" json:"address,omitempty" ssz-size:"20"`
	Amount  uint64 `protobuf:"varint,4,opt,name=amount,proto3" json:"amount,omitempty"`
	Node    []byte `protobuf:"bytes,5,opt,name=node,proto3" json:"node,omitempty" ssz-max:"20"`
}

func (x *TxInput) Reset() {
	*x = TxInput{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TxInput) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TxInput) ProtoMessage() {}

func (x *TxInput) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TxInput.ProtoReflect.Descriptor instead.
func (*TxInput) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{3}
}

func (x *TxInput) GetHash() []byte {
	if x != nil {
		return x.Hash
	}
	return nil
}

func (x *TxInput) GetIndex() uint32 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *TxInput) GetAddress() []byte {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *TxInput) GetAmount() uint64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

func (x *TxInput) GetNode() []byte {
	if x != nil {
		return x.Node
	}
	return nil
}

type TxOutput struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address []byte `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty" ssz-size:"20"`
	Amount  uint64 `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
	Node    []byte `protobuf:"bytes,3,opt,name=node,proto3" json:"node,omitempty" ssz-max:"20"`
}

func (x *TxOutput) Reset() {
	*x = TxOutput{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TxOutput) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TxOutput) ProtoMessage() {}

func (x *TxOutput) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TxOutput.ProtoReflect.Descriptor instead.
func (*TxOutput) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{4}
}

func (x *TxOutput) GetAddress() []byte {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *TxOutput) GetAmount() uint64 {
	if x != nil {
		return x.Amount
	}
	return 0
}

func (x *TxOutput) GetNode() []byte {
	if x != nil {
		return x.Node
	}
	return nil
}

type Metadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HeadSlot      uint64 `protobuf:"varint,1,opt,name=headSlot,proto3" json:"headSlot,omitempty"`
	HeadBlockNum  uint64 `protobuf:"varint,2,opt,name=headBlockNum,proto3" json:"headBlockNum,omitempty"`
	HeadBlockHash []byte `protobuf:"bytes,3,opt,name=headBlockHash,proto3" json:"headBlockHash,omitempty" ssz-size:"32"`
}

func (x *Metadata) Reset() {
	*x = Metadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata) ProtoMessage() {}

func (x *Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata.ProtoReflect.Descriptor instead.
func (*Metadata) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{5}
}

func (x *Metadata) GetHeadSlot() uint64 {
	if x != nil {
		return x.HeadSlot
	}
	return 0
}

func (x *Metadata) GetHeadBlockNum() uint64 {
	if x != nil {
		return x.HeadBlockNum
	}
	return 0
}

func (x *Metadata) GetHeadBlockHash() []byte {
	if x != nil {
		return x.HeadBlockHash
	}
	return nil
}

type BlockRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	StartSlot uint64 `protobuf:"varint,1,opt,name=startSlot,proto3" json:"startSlot,omitempty"`
	Count     uint64 `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Step      uint64 `protobuf:"varint,3,opt,name=step,proto3" json:"step,omitempty"`
}

func (x *BlockRequest) Reset() {
	*x = BlockRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BlockRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BlockRequest) ProtoMessage() {}

func (x *BlockRequest) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BlockRequest.ProtoReflect.Descriptor instead.
func (*BlockRequest) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{6}
}

func (x *BlockRequest) GetStartSlot() uint64 {
	if x != nil {
		return x.StartSlot
	}
	return 0
}

func (x *BlockRequest) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *BlockRequest) GetStep() uint64 {
	if x != nil {
		return x.Step
	}
	return 0
}

type Seed struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Seed     uint32 `protobuf:"varint,1,opt,name=seed,proto3" json:"seed,omitempty" ssz-max:"32"`
	Proposer *Sign  `protobuf:"bytes,2,opt,name=proposer,proto3" json:"proposer,omitempty"`
}

func (x *Seed) Reset() {
	*x = Seed{}
	if protoimpl.UnsafeEnabled {
		mi := &file_prototype_types_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Seed) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Seed) ProtoMessage() {}

func (x *Seed) ProtoReflect() protoreflect.Message {
	mi := &file_prototype_types_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Seed.ProtoReflect.Descriptor instead.
func (*Seed) Descriptor() ([]byte, []int) {
	return file_prototype_types_proto_rawDescGZIP(), []int{7}
}

func (x *Seed) GetSeed() uint32 {
	if x != nil {
		return x.Seed
	}
	return 0
}

func (x *Seed) GetProposer() *Sign {
	if x != nil {
		return x.Proposer
	}
	return nil
}

var File_prototype_types_proto protoreflect.FileDescriptor

var file_prototype_types_proto_rawDesc = []byte{
	0x0a, 0x15, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2f, 0x74, 0x79, 0x70, 0x65,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x13, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x1a, 0x20, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65,
	0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x11,
	0x65, 0x78, 0x74, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x17, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69,
	0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd1, 0x03, 0x0a, 0x05, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x10, 0x0a, 0x03, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x03, 0x6e, 0x75, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x12, 0x1f, 0x0a, 0x07, 0x76, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x05, 0x82, 0xb5, 0x18,
	0x01, 0x33, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x04, 0x68,
	0x61, 0x73, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x33,
	0x32, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x1e, 0x0a, 0x06, 0x70, 0x61, 0x72, 0x65, 0x6e,
	0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x33, 0x32, 0x52,
	0x06, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x18, 0x06, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x1e, 0x0a, 0x06, 0x74, 0x78, 0x72, 0x6f, 0x6f, 0x74, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x33, 0x32, 0x52, 0x06, 0x74,
	0x78, 0x72, 0x6f, 0x6f, 0x74, 0x12, 0x35, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65,
	0x72, 0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x69,
	0x67, 0x6e, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x12, 0x40, 0x0a, 0x09,
	0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x73, 0x18, 0x09, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x19, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e,
	0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x42, 0x07, 0x8a, 0xb5, 0x18, 0x03,
	0x31, 0x32, 0x38, 0x52, 0x09, 0x61, 0x70, 0x70, 0x72, 0x6f, 0x76, 0x65, 0x72, 0x73, 0x12, 0x3e,
	0x0a, 0x08, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x65, 0x72, 0x73, 0x18, 0x0a, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x19, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65,
	0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x42, 0x07, 0x8a, 0xb5, 0x18,
	0x03, 0x31, 0x32, 0x38, 0x52, 0x08, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x65, 0x72, 0x73, 0x12, 0x4e,
	0x0a, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x0b,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73,
	0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x08, 0x8a, 0xb5, 0x18, 0x04, 0x31, 0x35, 0x30, 0x30,
	0x52, 0x0c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0x4e,
	0x0a, 0x04, 0x53, 0x69, 0x67, 0x6e, 0x12, 0x20, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x82, 0xb5, 0x18, 0x02, 0x32, 0x30, 0x52,
	0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x24, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x82, 0xb5, 0x18,
	0x02, 0x36, 0x35, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x9d,
	0x03, 0x0a, 0x0b, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x19,
	0x0a, 0x03, 0x6e, 0x75, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x42, 0x07, 0xfa, 0x42, 0x04,
	0x32, 0x02, 0x28, 0x00, 0x52, 0x03, 0x6e, 0x75, 0x6d, 0x12, 0x1f, 0x0a, 0x04, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x42, 0x0b, 0xfa, 0x42, 0x08, 0x2a, 0x06, 0x30, 0x01,
	0x30, 0x05, 0x30, 0x06, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x25, 0x0a, 0x09, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x42, 0x07, 0xfa,
	0x42, 0x04, 0x32, 0x02, 0x20, 0x00, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x12, 0x21, 0x0a, 0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x42,
	0x0d, 0x82, 0xb5, 0x18, 0x02, 0x33, 0x32, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x20, 0x52, 0x04,
	0x68, 0x61, 0x73, 0x68, 0x12, 0x19, 0x0a, 0x03, 0x66, 0x65, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x04, 0x42, 0x07, 0xfa, 0x42, 0x04, 0x32, 0x02, 0x28, 0x00, 0x52, 0x03, 0x66, 0x65, 0x65, 0x12,
	0x25, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x11, 0x8a,
	0xb5, 0x18, 0x05, 0x31, 0x30, 0x30, 0x30, 0x30, 0xfa, 0x42, 0x05, 0x7a, 0x03, 0x18, 0x90, 0x4e,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x3e, 0x0a, 0x06, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73,
	0x18, 0x07, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x54, 0x78, 0x49,
	0x6e, 0x70, 0x75, 0x74, 0x42, 0x08, 0x8a, 0xb5, 0x18, 0x04, 0x32, 0x30, 0x30, 0x30, 0x52, 0x06,
	0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x12, 0x41, 0x0a, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74,
	0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x54, 0x78,
	0x4f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x42, 0x08, 0x8a, 0xb5, 0x18, 0x04, 0x32, 0x30, 0x30, 0x30,
	0x52, 0x07, 0x6f, 0x75, 0x74, 0x70, 0x75, 0x74, 0x73, 0x12, 0x2b, 0x0a, 0x09, 0x73, 0x69, 0x67,
	0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x0d, 0x82, 0xb5,
	0x18, 0x02, 0x36, 0x35, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x41, 0x52, 0x09, 0x73, 0x69, 0x67,
	0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x18, 0x0a, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0xb6,
	0x01, 0x0a, 0x07, 0x54, 0x78, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x12, 0x21, 0x0a, 0x04, 0x68, 0x61,
	0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x0d, 0x82, 0xb5, 0x18, 0x02, 0x33, 0x32,
	0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x20, 0x52, 0x04, 0x68, 0x61, 0x73, 0x68, 0x12, 0x14, 0x0a,
	0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x69, 0x6e,
	0x64, 0x65, 0x78, 0x12, 0x27, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0c, 0x42, 0x0d, 0x82, 0xb5, 0x18, 0x02, 0x32, 0x30, 0xfa, 0x42, 0x04, 0x7a,
	0x02, 0x68, 0x14, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1f, 0x0a, 0x06,
	0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x42, 0x07, 0xfa, 0x42,
	0x04, 0x32, 0x02, 0x20, 0x00, 0x52, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x28, 0x0a,
	0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x14, 0x8a, 0xb5, 0x18,
	0x02, 0x32, 0x30, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x14, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x70,
	0x01, 0x52, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x22, 0x7e, 0x0a, 0x08, 0x54, 0x78, 0x4f, 0x75, 0x74,
	0x70, 0x75, 0x74, 0x12, 0x27, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x42, 0x0d, 0x82, 0xb5, 0x18, 0x02, 0x32, 0x30, 0xfa, 0x42, 0x04, 0x7a,
	0x02, 0x68, 0x14, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1f, 0x0a, 0x06,
	0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x42, 0x07, 0xfa, 0x42,
	0x04, 0x32, 0x02, 0x20, 0x00, 0x52, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x28, 0x0a,
	0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x14, 0x8a, 0xb5, 0x18,
	0x02, 0x32, 0x30, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x14, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x70,
	0x01, 0x52, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x22, 0x7f, 0x0a, 0x08, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x65, 0x61, 0x64, 0x53, 0x6c, 0x6f, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x68, 0x65, 0x61, 0x64, 0x53, 0x6c, 0x6f, 0x74, 0x12,
	0x22, 0x0a, 0x0c, 0x68, 0x65, 0x61, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x4e, 0x75, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0c, 0x68, 0x65, 0x61, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x4e, 0x75, 0x6d, 0x12, 0x33, 0x0a, 0x0d, 0x68, 0x65, 0x61, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x48, 0x61, 0x73, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x0d, 0x8a, 0xb5, 0x18, 0x02,
	0x33, 0x32, 0xfa, 0x42, 0x04, 0x7a, 0x02, 0x68, 0x20, 0x52, 0x0d, 0x68, 0x65, 0x61, 0x64, 0x42,
	0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x22, 0x56, 0x0a, 0x0c, 0x42, 0x6c, 0x6f, 0x63,
	0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x74, 0x61, 0x72,
	0x74, 0x53, 0x6c, 0x6f, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x53, 0x6c, 0x6f, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04,
	0x73, 0x74, 0x65, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x74, 0x65, 0x70,
	0x22, 0x51, 0x0a, 0x04, 0x53, 0x65, 0x65, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x65, 0x65, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x73, 0x65, 0x65, 0x64, 0x12, 0x35, 0x0a, 0x08,
	0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19,
	0x2e, 0x72, 0x64, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x2e, 0x74,
	0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x70, 0x6f,
	0x73, 0x65, 0x72, 0x42, 0x17, 0x5a, 0x15, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65,
	0x2f, 0x2e, 0x3b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x74, 0x79, 0x70, 0x65, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_prototype_types_proto_rawDescOnce sync.Once
	file_prototype_types_proto_rawDescData = file_prototype_types_proto_rawDesc
)

func file_prototype_types_proto_rawDescGZIP() []byte {
	file_prototype_types_proto_rawDescOnce.Do(func() {
		file_prototype_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_prototype_types_proto_rawDescData)
	})
	return file_prototype_types_proto_rawDescData
}

var file_prototype_types_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_prototype_types_proto_goTypes = []interface{}{
	(*Block)(nil),        // 0: rdo.prototype.types.Block
	(*Sign)(nil),         // 1: rdo.prototype.types.Sign
	(*Transaction)(nil),  // 2: rdo.prototype.types.Transaction
	(*TxInput)(nil),      // 3: rdo.prototype.types.TxInput
	(*TxOutput)(nil),     // 4: rdo.prototype.types.TxOutput
	(*Metadata)(nil),     // 5: rdo.prototype.types.Metadata
	(*BlockRequest)(nil), // 6: rdo.prototype.types.BlockRequest
	(*Seed)(nil),         // 7: rdo.prototype.types.Seed
}
var file_prototype_types_proto_depIdxs = []int32{
	1, // 0: rdo.prototype.types.Block.proposer:type_name -> rdo.prototype.types.Sign
	1, // 1: rdo.prototype.types.Block.approvers:type_name -> rdo.prototype.types.Sign
	1, // 2: rdo.prototype.types.Block.slashers:type_name -> rdo.prototype.types.Sign
	2, // 3: rdo.prototype.types.Block.transactions:type_name -> rdo.prototype.types.Transaction
	3, // 4: rdo.prototype.types.Transaction.inputs:type_name -> rdo.prototype.types.TxInput
	4, // 5: rdo.prototype.types.Transaction.outputs:type_name -> rdo.prototype.types.TxOutput
	1, // 6: rdo.prototype.types.Seed.proposer:type_name -> rdo.prototype.types.Sign
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_prototype_types_proto_init() }
func file_prototype_types_proto_init() {
	if File_prototype_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_prototype_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Block); i {
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
		file_prototype_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Sign); i {
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
		file_prototype_types_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Transaction); i {
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
		file_prototype_types_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TxInput); i {
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
		file_prototype_types_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TxOutput); i {
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
		file_prototype_types_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metadata); i {
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
		file_prototype_types_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BlockRequest); i {
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
		file_prototype_types_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Seed); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_prototype_types_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_prototype_types_proto_goTypes,
		DependencyIndexes: file_prototype_types_proto_depIdxs,
		MessageInfos:      file_prototype_types_proto_msgTypes,
	}.Build()
	File_prototype_types_proto = out.File
	file_prototype_types_proto_rawDesc = nil
	file_prototype_types_proto_goTypes = nil
	file_prototype_types_proto_depIdxs = nil
}
