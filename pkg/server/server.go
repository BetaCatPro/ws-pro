package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/BetaCatPro/ws-pro/internal/conn"
	"github.com/BetaCatPro/ws-pro/internal/errors"
	"github.com/BetaCatPro/ws-pro/pkg/types"
	"github.com/gorilla/websocket"
)

// Server WebSocket服务器
type Server struct {
    addr         string
    config       *types.Config
    connManager  *conn.ConnectionManager
    upgrader     websocket.Upgrader
    server       *http.Server
    mutex        sync.RWMutex
    
    // 回调函数
    connectHandler    func(string)
    disconnectHandler func(string, error)
    messageHandler    func(string, types.Message)
}

// NewServer 创建新的WebSocket服务器
func NewServer(addr string, config *types.Config) *Server {
    return &Server{
        addr:        addr,
        config:      config,
        connManager: conn.NewConnectionManager(config),
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool { return true },
        },
    }
}

// Start 启动WebSocket服务器
func (s *Server) Start() error {
    mux := http.NewServeMux()
    mux.HandleFunc("/ws", s.handleWebSocket)
    
    s.server = &http.Server{
        Addr:    s.addr,
        Handler: mux,
    }
    
    fmt.Printf("Starting WebSocket server on %s\n", s.addr)
    return s.server.ListenAndServe()
}

// handleWebSocket 处理WebSocket连接
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    wsConn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    errorCenter := errors.NewErrorCenter()
    reconnectMgr := conn.NewReconnectManager(s.config, errorCenter)
    
    connection := conn.NewConnection(wsConn, s.config, errorCenter, reconnectMgr)
    connectionID := connection.GetID()

    // 设置连接的消息处理回调
    connection.SetMessageHandler(func(msg types.Message) {
        s.handleClientMessage(connectionID, msg)
    })
    
    s.connManager.AddConnection(connectionID, connection)
    connection.Start()
    
    // 触发连接成功回调
    if s.connectHandler != nil {
        s.connectHandler(connectionID)
    }

    // 设置连接关闭时的清理操作
    wsConn.SetCloseHandler(func(code int, text string) error {
        s.connManager.RemoveConnection(connectionID)
        
        // 触发断开连接回调
        if s.disconnectHandler != nil {
            s.disconnectHandler(connectionID, fmt.Errorf("connection closed: %s", text))
        }
        
        return nil
    })
}

// BroadcastMessage 广播消息到所有客户端
func (s *Server) BroadcastMessage(msg types.Message) {
    s.connManager.Broadcast(msg)
}

// SendToClient 发送消息到指定客户端
func (s *Server) SendToClient(connectionID string, msg types.Message) error {
    if conn, exists := s.connManager.GetConnection(connectionID); exists {
        return conn.Send(msg)
    }
    return errors.ErrConnectionClosed
}

// SetConnectHandler 设置连接成功回调
func (s *Server) SetConnectHandler(handler func(string)) {
    s.connectHandler = handler
}

// SetDisconnectHandler 设置断开连接回调
func (s *Server) SetDisconnectHandler(handler func(string, error)) {
    s.disconnectHandler = handler
}

// SetMessageHandler 设置消息处理回调
func (s *Server) SetMessageHandler(handler func(string, types.Message)) {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    s.messageHandler = handler
}

// handleClientMessage 处理客户端消息
func (s *Server) handleClientMessage(connectionID string, msg types.Message) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    // 调用消息处理回调
    if s.messageHandler != nil {
        s.messageHandler(connectionID, msg)
    }
}

// GetStats 获取服务器统计信息
func (s *Server) GetStats() types.ConnectionStats {
    return s.connManager.GetStats()
}

// GetClientCount 获取客户端数量
func (s *Server) GetClientCount() int {
    connections := s.connManager.GetAllConnections()
    return len(connections)
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
    s.connManager.CloseAll()
    return s.server.Shutdown(ctx)
}