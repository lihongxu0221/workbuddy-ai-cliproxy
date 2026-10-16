package cpa

import (
	"encoding/json"
	"fmt"
)

// HostBridge 宿主回调接口，供异步流式推流与宿主交互
type HostBridge interface {
	HostCall(method string, request []byte) ([]byte, error)
	StreamEmit(streamID string, payload []byte) error
	StreamClose(streamID string)
}

// NoopHostBridge 用于独立模式或未挂载宿主时的空实现
type NoopHostBridge struct{}

func (n *NoopHostBridge) HostCall(method string, request []byte) ([]byte, error) {
	return nil, fmt.Errorf("host bridge not connected")
}

func (n *NoopHostBridge) StreamEmit(streamID string, payload []byte) error {
	return nil
}

func (n *NoopHostBridge) StreamClose(streamID string) {}

// GenericHostBridge 基于通用回调函数的桥接实现
type GenericHostBridge struct {
	CallFn func(method string, request []byte) ([]byte, error)
}

func (b *GenericHostBridge) HostCall(method string, request []byte) ([]byte, error) {
	if b.CallFn == nil {
		return nil, fmt.Errorf("call function is nil")
	}
	return b.CallFn(method, request)
}

func (b *GenericHostBridge) StreamEmit(streamID string, payload []byte) error {
	if streamID == "" {
		return fmt.Errorf("stream id is empty")
	}
	body, _ := json.Marshal(map[string]any{
		"stream_id": streamID,
		"payload":   payload,
	})
	_, err := b.HostCall(MethodHostStreamEmit, body)
	return err
}

func (b *GenericHostBridge) StreamClose(streamID string) {
	if streamID == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"stream_id": streamID,
	})
	_, _ = b.HostCall(MethodHostStreamClose, body)
}
