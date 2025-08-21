package conn

import (
	"fmt"
	"sync"
	"time"

	"github.com/BetaCatPro/ws-pro/internal/compression"
	"github.com/BetaCatPro/ws-pro/internal/errors"
	"github.com/BetaCatPro/ws-pro/internal/protocol"
	"github.com/BetaCatPro/ws-pro/internal/utils"
	"github.com/BetaCatPro/ws-pro/pkg/types"
	"github.com/gorilla/websocket"
	"go.uber.org/atomic"
)

// Connection WebSocket连接
type Connection struct {
    conn         *websocket.Conn          // 底层WebSocket连接
    config       *types.Config            // 配置信息
    errorCenter  *errors.ErrorCenter      // 错误处理中心
    compressor   compression.Compressor   // 压缩器
    protocol     protocol.MessageProtocol // 协议处理器
    reconnectMgr *ReconnectManager        // 重连管理器

    sendMutex    sync.Mutex               // 发送锁
    receiveMutex sync.Mutex               // 接收锁

    isConnected      atomic.Bool          // 连接状态原子变量
    messageQueue     chan types.Message   // 消息发送队列
    done             chan struct{}        // 结束信号
    heartbeatTimeout chan struct{}        // 心跳超时信号

    stats types.ConnectionStats           // 连接统计信息

    // 重连相关字段
    url      string                       // 连接URL
    headers  map[string]string            // 请求头
    id       string                       // 连接ID

    // 消息处理回调
    messageHandler func(types.Message)    // 消息处理回调函数
}

// NewConnection 创建新的WebSocket连接
func NewConnection(wsConn *websocket.Conn, config *types.Config, 
                  errorCenter *errors.ErrorCenter, reconnectMgr *ReconnectManager) *Connection {
    conn := &Connection{
        conn:             wsConn,
        config:           config,
        messageQueue:     make(chan types.Message, config.BufferSize),
        done:             make(chan struct{}),
        heartbeatTimeout: make(chan struct{}, 1),
        errorCenter:      errorCenter,
        compressor:       compression.GetCompressor("gzip"),
        protocol:         protocol.GetProtocol("json"),
        reconnectMgr:     reconnectMgr,
        headers:          make(map[string]string),
        id:               generateConnectionID(),
    }
    conn.isConnected.Store(true)
    return conn
}

// SetMessageHandler 设置消息处理回调
func (c *Connection) SetMessageHandler(handler func(types.Message)) {
    c.messageHandler = handler
}

// Start 启动连接处理协程
func (c *Connection) Start() {
    go c.heartbeat()          // 心跳协程
    go c.heartbeatMonitor()   // 心跳监控协程
    go c.processMessages()    // 消息处理协程
    go c.readMessages()       // 消息读取协程
}

// processMessages 处理发送队列中的消息
func (c *Connection) processMessages() {
    for {
        select {
        case msg := <-c.messageQueue:
            if err := c.writeMessage(msg); err != nil {
                c.errorCenter.ReportError(fmt.Errorf("failed to send message: %v", err))
                // 只有非心跳消息发送失败才触发重连
                if msg.Type != websocket.PingMessage {
                    c.handleDisconnection()
                    return
                }
            }
            c.stats.TotalMessages++
        case <-c.done:
            return
        }
    }
}

// writeMessage 写入消息到WebSocket连接
func (c *Connection) writeMessage(msg types.Message) error {
    c.sendMutex.Lock()
    defer c.sendMutex.Unlock()

    var dataToSend []byte
    var err error

    // 处理不同类型的消息
    switch msg.Type {
    case websocket.PingMessage, websocket.PongMessage, websocket.CloseMessage:
        // 控制消息：直接发送，不压缩
        dataToSend = msg.Content
    case websocket.TextMessage, websocket.BinaryMessage:
        // 业务消息：需要压缩
        dataToSend, err = c.compressor.Compress(msg.Content)
        if err != nil {
            return fmt.Errorf("compress failed: %v", err)
        }
    default:
        return fmt.Errorf("unsupported message type: %d", msg.Type)
    }

    // 设置写超时
    c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
    
    if msg.Type == websocket.PingMessage || msg.Type == websocket.PongMessage || msg.Type == websocket.CloseMessage {
        // 使用 WriteControl 发送控制消息
        err = c.conn.WriteControl(msg.Type, dataToSend, time.Now().Add(5*time.Second))
    } else {
        // 使用 WriteMessage 发送数据消息
        err = c.conn.WriteMessage(msg.Type, dataToSend)
    }
    
    if err != nil {
        return fmt.Errorf("write message failed: %v", err)
    }

    return nil
}

// readMessages 读取来自WebSocket的消息
func (c *Connection) readMessages() {
    defer c.handleDisconnection()

    for {
        if !c.isConnected.Load() {
            return
        }

        // 设置读超时
        c.conn.SetReadDeadline(time.Now().Add(c.config.HeartbeatInterval * 3))
        
        msgType, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
                c.errorCenter.ReportError(fmt.Errorf("read message error: %v", err))
            }
            return
        }

        // 处理控制消息
        switch msgType {
        case websocket.PongMessage:
            // 收到pong，重置心跳超时
            select {
            case c.heartbeatTimeout <- struct{}{}:
            default:
            }
            continue
        case websocket.PingMessage:
            // 回应ping
            c.conn.WriteControl(websocket.PongMessage, message, time.Now().Add(5*time.Second))
            continue
        case websocket.CloseMessage:
            // 处理关闭消息
            return
        }

        // 处理业务消息
        c.handleMessage(msgType, message)
    }
}

// handleMessage 处理接收到的业务消息
func (c *Connection) handleMessage(msgType int, message []byte) {
    // 只处理文本和二进制消息
    if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
        // 忽略控制消息
        return
    }

    // 尝试解压缩（如果是压缩的消息）
    var decompressedData []byte
    var err error
    
    // 检查是否是压缩数据（简单的启发式检查）
    if isLikelyCompressed(message) {
        decompressedData, err = c.compressor.Decompress(message)
        if err != nil {
            c.errorCenter.ReportError(fmt.Errorf("decompress failed: %v", err))
            // 即使解压缩失败，也尝试处理原始消息
            decompressedData = message
        }
    } else {
        decompressedData = message
    }

    // 创建消息对象
    msg := types.Message{
        Type:    msgType,
        Content: decompressedData,
        ID:      utils.GenerateMessageID(),
    }

    // 记录接收到的消息
    c.stats.TotalMessages++

    // 调用消息处理回调
    if c.messageHandler != nil {
        c.messageHandler(msg)
    }
}

// isLikelyCompressed 简单的启发式检查是否是压缩数据
func isLikelyCompressed(data []byte) bool {
    if len(data) < 2 {
        return false
    }
    
    // 检查gzip魔数
    if data[0] == 0x1F && data[1] == 0x8B {
        return true
    }
    
    // 检查snappy特征（简单的启发式）
    if len(data) > 10 {
        // Snappy流式格式可能有特定的模式
        // 这里使用简单的长度和内容检查
        return true
    }
    
    return false
}

// heartbeat 发送心跳
func (c *Connection) heartbeat() {
    ticker := time.NewTicker(c.config.HeartbeatInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if !c.isConnected.Load() {
                return
            }

            // 发送心跳
            if err := c.conn.WriteControl(websocket.PingMessage, 
                []byte(c.config.HeartbeatMsg), time.Now().Add(5*time.Second)); err != nil {
                c.errorCenter.ReportError(fmt.Errorf("heartbeat send failed: %v", err))
                // 心跳发送失败，记录错误但不触发重连（让心跳超时检测来处理）
                // c.handleDisconnection()
                // return
                continue
            }

            // 重置心跳超时检测
            select {
            case c.heartbeatTimeout <- struct{}{}:
            default:
                // 避免阻塞
            }

        case <-c.done:
            return
        }
    }
}

// heartbeatMonitor 监控心跳超时
func (c *Connection) heartbeatMonitor() {
    timeout := c.config.HeartbeatInterval * 3 // 三倍心跳间隔作为超时时间
    timer := time.NewTimer(timeout)
    defer timer.Stop()

    for {
        select {
        case <-c.heartbeatTimeout:
            if !timer.Stop() {
                select {
                case <-timer.C:
                default:
                }
            }
            timer.Reset(timeout)
        case <-timer.C:
            // 心跳超时，触发重连
            c.errorCenter.ReportError(fmt.Errorf("heartbeat timeout, triggering reconnect"))
            c.handleDisconnection()
            return
        case <-c.done:
            return
        }
    }
}

// handleDisconnection 处理连接断开
func (c *Connection) handleDisconnection() {
    if !c.isConnected.Load() {
        return
    }

    c.isConnected.Store(false)
    close(c.done)
    c.conn.Close()
    
    // 触发重连
    if c.reconnectMgr != nil {
        c.reconnectMgr.ScheduleReconnect(c)
    }
}

// Send 发送消息到队列
func (c *Connection) Send(msg types.Message) error {
    if !c.isConnected.Load() {
        return errors.ErrConnectionClosed
    }
    
    select {
    case c.messageQueue <- msg:
        return nil
    default:
        c.stats.DroppedMessages++
        return errors.ErrBufferFull
    }
}

// IsConnected 检查连接状态
func (c *Connection) IsConnected() bool {
    return c.isConnected.Load()
}

// SetConnectionInfo 设置连接信息用于重连
func (c *Connection) SetConnectionInfo(url string, headers map[string]string) {
    c.url = url
    c.headers = headers
}

// GetStats 获取连接统计信息
func (c *Connection) GetStats() types.ConnectionStats {
    return c.stats
}

// GetID 获取连接ID
func (c *Connection) GetID() string {
    return c.id
}

// Close 主动关闭连接
func (c *Connection) Close() {
    c.isConnected.Store(false)
    close(c.done)
    c.conn.Close()
}

// generateConnectionID 生成唯一连接ID
func generateConnectionID() string {
    return fmt.Sprintf("conn-%d", time.Now().UnixNano())
}