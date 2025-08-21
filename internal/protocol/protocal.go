package protocol

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// MessageProtocol 消息协议接口
type MessageProtocol interface {
    Encode(interface{}) ([]byte, error)     // 编码消息
    Decode([]byte, interface{}) error       // 解码消息
}

// JSONProtocol JSON协议实现
type JSONProtocol struct{}

// Encode 编码为JSON
func (j *JSONProtocol) Encode(data interface{}) ([]byte, error) {
    return json.Marshal(data)
}

// Decode 从JSON解码
func (j *JSONProtocol) Decode(bytes []byte, target interface{}) error {
    return json.Unmarshal(bytes, target)
}

// ProtobufProtocol Protobuf协议实现
type ProtobufProtocol struct{}

// Encode 编码为Protobuf
func (p *ProtobufProtocol) Encode(data interface{}) ([]byte, error) {
    if msg, ok := data.(proto.Message); ok {
        return proto.Marshal(msg)
    }
    return nil, fmt.Errorf("data is not a proto.Message")
}

// Decode 从Protobuf解码
func (p *ProtobufProtocol) Decode(bytes []byte, target interface{}) error {
    if msg, ok := target.(proto.Message); ok {
        return proto.Unmarshal(bytes, msg)
    }
    return fmt.Errorf("target is not a proto.Message")
}

// GetProtocol 根据名称获取协议处理器
func GetProtocol(name string) MessageProtocol {
    switch name {
    case "protobuf":
        return &ProtobufProtocol{}
    case "json":
    default:
        return &JSONProtocol{}
    }
    return &JSONProtocol{}
}