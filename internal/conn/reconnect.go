package conn

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/BetaCatPro/ws-pro/internal/errors"
	"github.com/BetaCatPro/ws-pro/pkg/types"
	"github.com/gorilla/websocket"
)

// reconnectRequest 重连请求
type reconnectRequest struct {
    conn    *Connection
    attempt int
}

// ReconnectManager 重连管理器
type ReconnectManager struct {
    config          *types.Config
    errorCenter     *errors.ErrorCenter
    reconnectSignal chan *reconnectRequest
    stopSignal      chan struct{}
}

// NewReconnectManager 创建重连管理器
func NewReconnectManager(config *types.Config, errorCenter *errors.ErrorCenter) *ReconnectManager {
    return &ReconnectManager{
        config:          config,
        errorCenter:     errorCenter,
        reconnectSignal: make(chan *reconnectRequest, 100),
        stopSignal:      make(chan struct{}),
    }
}

// Start 启动重连管理器
func (rm *ReconnectManager) Start() {
    go rm.processReconnects()
}

// Stop 停止重连管理器
func (rm *ReconnectManager) Stop() {
    close(rm.stopSignal)
}

// ScheduleReconnect 调度重连
func (rm *ReconnectManager) ScheduleReconnect(conn *Connection) {
    req := &reconnectRequest{
        conn:    conn,
        attempt: 0,
    }
    
    select {
    case rm.reconnectSignal <- req:
    default:
        rm.errorCenter.ReportError(fmt.Errorf("reconnect queue full, dropping connection"))
    }
}

// processReconnects 处理重连请求
func (rm *ReconnectManager) processReconnects() {
    for {
        select {
        case req := <-rm.reconnectSignal:
            go rm.reconnectWithBackoff(req)
        case <-rm.stopSignal:
            return
        }
    }
}

// reconnectWithBackoff 使用指数退避算法进行重连
func (rm *ReconnectManager) reconnectWithBackoff(req *reconnectRequest) {
    maxAttempts := rm.config.MaxReconnectTimes

    for req.attempt < maxAttempts {
        waitTime := rm.calculateBackoff(req.attempt)
        
        rm.errorCenter.ReportError(fmt.Errorf("reconnect attempt %d/%d, waiting %v", 
            req.attempt+1, maxAttempts, waitTime))

        select {
        case <-time.After(waitTime):
            if rm.tryReconnect(req.conn) {
                rm.errorCenter.ReportError(fmt.Errorf("reconnect successful after %d attempts", req.attempt+1))
                return
            }
            req.attempt++
        case <-rm.stopSignal:
            return
        }
    }

    rm.errorCenter.ReportError(errors.ErrMaxReconnect)
}

// calculateBackoff 计算退避时间
func (rm *ReconnectManager) calculateBackoff(attempt int) time.Duration {
    baseTime := rm.config.ReconnectBaseTime
    waitTime := baseTime * time.Duration(1<<uint(attempt)) // 指数退避
    jitter := time.Duration(rand.Int63n(int64(waitTime/2))) // 添加随机抖动
    return waitTime + jitter
}

// tryReconnect 尝试重连
func (rm *ReconnectManager) tryReconnect(conn *Connection) bool {
    if conn.url == "" {
        return false
    }

    dialer := &websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
    }

    // 创建请求头
    header := http.Header{}
    for key, value := range conn.headers {
        header.Set(key, value)
    }

    wsConn, _, err := dialer.Dial(conn.url, header)
    if err != nil {
        rm.errorCenter.ReportError(fmt.Errorf("reconnect dial failed: %v", err))
        return false
    }

    // 重新设置连接
    conn.conn = wsConn
    conn.isConnected.Store(true)
    conn.done = make(chan struct{})
    conn.heartbeatTimeout = make(chan struct{}, 1)

    // 重新启动协程
    go conn.heartbeat()
    go conn.heartbeatMonitor()
    go conn.processMessages()
    go conn.readMessages()

    return true
}