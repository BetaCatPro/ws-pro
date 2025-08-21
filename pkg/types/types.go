package types

import (
	"time"
)

// Config WebSocket配置结构体
type Config struct {
    HeartbeatInterval time.Duration // 心跳间隔
    HeartbeatMsg      string        // 心跳消息内容
    ReconnectBaseTime time.Duration // 重连基础时间
    MaxReconnectTimes int           // 最大重连次数
    BufferSize        int           // 消息缓冲区大小
    MaxConcurrent     int           // 最大并发连接数
}

// Message 消息结构体
type Message struct {
    Type    int    // 消息类型（TextMessage, BinaryMessage等）
    Content []byte // 消息内容
    ID      string // 消息ID（用于去重和确认）
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
    ActiveConnections int   // 活跃连接数
    TotalMessages     int64 // 总消息数
    DroppedMessages   int64 // 丢弃消息数
    ReconnectAttempts int   // 重连尝试次数
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
    ID      string            // 连接ID
    URL     string            // 连接URL
    Headers map[string]string // 请求头
    Stats   ConnectionStats   // 统计信息
}