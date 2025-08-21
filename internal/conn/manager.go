package conn

import (
	"sync"

	"github.com/BetaCatPro/ws-pro/internal/errors"
	"github.com/BetaCatPro/ws-pro/pkg/types"
)

// ConnectionManager 连接管理器
type ConnectionManager struct {
    connections  map[string]*Connection
    config       *types.Config
    mutex        sync.RWMutex
    errorCenter  *errors.ErrorCenter
    reconnectMgr *ReconnectManager
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config *types.Config) *ConnectionManager {
    errorCenter := errors.NewErrorCenter()
    reconnectMgr := NewReconnectManager(config, errorCenter)
    
    manager := &ConnectionManager{
        connections:  make(map[string]*Connection),
        config:       config,
        errorCenter:  errorCenter,
        reconnectMgr: reconnectMgr,
    }
    
    reconnectMgr.Start()
    return manager
}

// AddConnection 添加连接
func (cm *ConnectionManager) AddConnection(id string, conn *Connection) {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()
    cm.connections[id] = conn
}

// RemoveConnection 移除连接
func (cm *ConnectionManager) RemoveConnection(id string) {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()
    if conn, exists := cm.connections[id]; exists {
        conn.Close()
        delete(cm.connections, id)
    }
}

// GetConnection 获取连接
func (cm *ConnectionManager) GetConnection(id string) (*Connection, bool) {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()
    conn, exists := cm.connections[id]
    return conn, exists
}

// GetAllConnections 获取所有连接
func (cm *ConnectionManager) GetAllConnections() map[string]*Connection {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()
    return cm.connections
}

// Broadcast 广播消息到所有连接
func (cm *ConnectionManager) Broadcast(msg types.Message) {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()
    
    for _, conn := range cm.connections {
        if conn.IsConnected() {
            conn.Send(msg)
        }
    }
}

// GetStats 获取连接统计信息
func (cm *ConnectionManager) GetStats() types.ConnectionStats {
    cm.mutex.RLock()
    defer cm.mutex.RUnlock()
    
    stats := types.ConnectionStats{}
    for _, conn := range cm.connections {
        connStats := conn.GetStats()
        stats.ActiveConnections++
        stats.TotalMessages += connStats.TotalMessages
        stats.DroppedMessages += connStats.DroppedMessages
    }
    return stats
}

// CloseAll 关闭所有连接
func (cm *ConnectionManager) CloseAll() {
    cm.mutex.Lock()
    defer cm.mutex.Unlock()
    
    for id, conn := range cm.connections {
        conn.Close()
        delete(cm.connections, id)
    }
}