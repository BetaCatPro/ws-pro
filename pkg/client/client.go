package client

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/BetaCatPro/ws-pro/internal/conn"
	"github.com/BetaCatPro/ws-pro/internal/errors"
	"github.com/BetaCatPro/ws-pro/pkg/types"
	"github.com/gorilla/websocket"
)

// Client WebSocket客户端
type Client struct {
    url          string
    config       *types.Config
    connManager  *conn.ConnectionManager
    errorCenter  *errors.ErrorCenter
    reconnectMgr *conn.ReconnectManager
    headers      map[string]string
    mutex        sync.RWMutex
    
    // 回调函数
    messageHandler   func(types.Message)
    connectHandler   func()
    disconnectHandler func(error)
}

// NewClient 创建新的WebSocket客户端
func NewClient(serverURL string, config *types.Config) *Client {
    errorCenter := errors.NewErrorCenter()
    reconnectMgr := conn.NewReconnectManager(config, errorCenter)
    
    client := &Client{
        url:          serverURL,
        config:       config,
        connManager:  conn.NewConnectionManager(config),
        errorCenter:  errorCenter,
        reconnectMgr: reconnectMgr,
        headers:      make(map[string]string),
    }
    
    // 启动重连管理器
    reconnectMgr.Start()
    
    // 设置错误回调
    errorCenter.AddErrorCallback(client.handleError)
    
    return client
}

// Connect 连接到WebSocket服务器
func (c *Client) Connect() error {
    u, err := url.Parse(c.url)
    if err != nil {
        return err
    }

    dialer := &websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
    }

    // 创建请求头
    header := http.Header{}
    for key, value := range c.headers {
        header.Set(key, value)
    }

    wsConn, _, err := dialer.Dial(u.String(), header)
    if err != nil {
        return err
    }

    connection := conn.NewConnection(wsConn, c.config, c.errorCenter, c.reconnectMgr)
    connection.SetConnectionInfo(c.url, c.headers)

    // 设置消息处理回调
    if c.messageHandler != nil {
        connection.SetMessageHandler(c.messageHandler)
    }
    
    c.connManager.AddConnection("default", connection)
    connection.Start()

    // 触发连接成功回调
    if c.connectHandler != nil {
        c.connectHandler()
    }

    return nil
}

// SendMessage 发送消息
func (c *Client) SendMessage(msg types.Message) error {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if conn, exists := c.connManager.GetConnection("default"); exists {
        return conn.Send(msg)
    }
    return errors.ErrConnectionClosed
}

// SendPing 发送心跳消息（如果需要手动发送心跳）
func (c *Client) SendPing() error {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if conn, exists := c.connManager.GetConnection("default"); exists {
        pingMsg := types.Message{
            Type:    websocket.PingMessage,
            Content: []byte(c.config.HeartbeatMsg),
        }
        return conn.Send(pingMsg)
    }
    return errors.ErrConnectionClosed
}

// SetHeader 设置请求头
func (c *Client) SetHeader(key, value string) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.headers[key] = value
}

// SetMessageHandler 设置消息处理回调
func (c *Client) SetMessageHandler(handler func(types.Message)) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.messageHandler = handler
    
    // 如果已经有连接，设置消息处理回调
    if conn, exists := c.connManager.GetConnection("default"); exists {
        conn.SetMessageHandler(handler)
    }
}

// SetConnectHandler 设置连接成功回调
func (c *Client) SetConnectHandler(handler func()) {
    c.connectHandler = handler
}

// SetDisconnectHandler 设置断开连接回调
func (c *Client) SetDisconnectHandler(handler func(error)) {
    c.disconnectHandler = handler
}

// handleError 处理错误
func (c *Client) handleError(err error) {
    // 过滤掉重连成功的消息，不触发断开连接回调
    if isReconnectSuccessError(err) {
        fmt.Printf("重连成功: %v\n", err)
        return
    }
    
    fmt.Printf("客户端错误: %v\n", err)
    
    // 触发断开连接回调（排除重连相关的成功消息）
    if c.disconnectHandler != nil && !isReconnectRelatedError(err) {
        c.disconnectHandler(err)
    }
    
    // 如果是连接错误，尝试重连
    if err == errors.ErrConnectionClosed {
        c.mutex.Lock()
        defer c.mutex.Unlock()
        
        if conn, exists := c.connManager.GetConnection("default"); !exists || !conn.IsConnected() {
            go func() {
                time.Sleep(1 * time.Second) // 稍等片刻再重连
                if err := c.Connect(); err != nil {
                    c.errorCenter.ReportError(fmt.Errorf("auto reconnect failed: %v", err))
                }
            }()
        }
    }
}

// isReconnectSuccessError 检查是否是重连成功的错误
func isReconnectSuccessError(err error) bool {
    if err == nil {
        return false
    }
    errorMsg := err.Error()
    return contains(errorMsg, "reconnect successful") || 
           contains(errorMsg, "重连成功")
}

// isReconnectRelatedError 检查是否是重连相关的错误
func isReconnectRelatedError(err error) bool {
    if err == nil {
        return false
    }
    errorMsg := err.Error()
    return contains(errorMsg, "reconnect") || 
           contains(errorMsg, "重连") ||
           contains(errorMsg, "heartbeat timeout")
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
    return len(s) >= len(substr) && s[:len(substr)] == substr
}

// IsConnected 检查连接状态
func (c *Client) IsConnected() bool {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if conn, exists := c.connManager.GetConnection("default"); exists {
        return conn.IsConnected()
    }
    return false
}

// Close 关闭客户端
func (c *Client) Close() {
    c.reconnectMgr.Stop()
    c.connManager.RemoveConnection("default")
}

// GetStats 获取统计信息
func (c *Client) GetStats() types.ConnectionStats {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if conn, exists := c.connManager.GetConnection("default"); exists {
        return conn.GetStats()
    }
    return types.ConnectionStats{}
}

// GetConnectionInfo 获取连接信息
func (c *Client) GetConnectionInfo() types.ConnectionInfo {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    if conn, exists := c.connManager.GetConnection("default"); exists {
        return types.ConnectionInfo{
            ID:      conn.GetID(),
            URL:     c.url,
            Headers: c.headers,
            Stats:   conn.GetStats(),
        }
    }
    return types.ConnectionInfo{}
}