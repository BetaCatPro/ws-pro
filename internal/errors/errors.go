package errors

import "fmt"

// 定义错误类型
var (
    ErrConnectionClosed   = fmt.Errorf("connection closed")
    ErrMaxReconnect       = fmt.Errorf("max reconnect times reached")
    ErrBufferFull         = fmt.Errorf("message buffer full")
    ErrInvalidMessage     = fmt.Errorf("invalid message")
    ErrCompressionFailed  = fmt.Errorf("compression failed")
    ErrDecompressionFailed = fmt.Errorf("decompression failed")
    ErrProtocolError      = fmt.Errorf("protocol error")
)

// ErrorCenter 错误处理中心
type ErrorCenter struct {
    errorCallbacks []func(error) // 错误回调函数列表
}

// NewErrorCenter 创建新的错误处理中心
func NewErrorCenter() *ErrorCenter {
    return &ErrorCenter{
        errorCallbacks: make([]func(error), 0),
    }
}

// AddErrorCallback 添加错误回调函数
func (ec *ErrorCenter) AddErrorCallback(callback func(error)) {
    ec.errorCallbacks = append(ec.errorCallbacks, callback)
}

// ReportError 报告错误
func (ec *ErrorCenter) ReportError(err error) {
    for _, callback := range ec.errorCallbacks {
        callback(err)
    }
}

// ClearCallbacks 清空所有回调函数
func (ec *ErrorCenter) ClearCallbacks() {
    ec.errorCallbacks = make([]func(error), 0)
}