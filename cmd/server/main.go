package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BetaCatPro/ws-pro/pkg/server"
	"github.com/BetaCatPro/ws-pro/pkg/types"
)

func main() {
    config := &types.Config{
        HeartbeatInterval: 30 * time.Second,
        HeartbeatMsg:      "pong",
        ReconnectBaseTime: 1 * time.Second,
        MaxReconnectTimes: 5,
        BufferSize:        10000,
        MaxConcurrent:     1000,
    }
    
    // 创建服务器
    wsServer := server.NewServer(":8080", config)
    
    // 设置连接成功回调
    wsServer.SetConnectHandler(func(connID string) {
        fmt.Printf("客户端连接: %s\n", connID)
        
        // 发送欢迎消息
        welcomeMsg := types.Message{
            Type:    1,
            Content: []byte(fmt.Sprintf("欢迎连接 WebSocket 服务器! 你的连接ID: %s", connID)),
        }
        if err := wsServer.SendToClient(connID, welcomeMsg); err != nil {
            log.Printf("发送欢迎消息失败: %v", err)
        }
    })
    
    // 设置断开连接回调
    wsServer.SetDisconnectHandler(func(connID string, err error) {
        fmt.Printf("客户端断开: %s, 原因: %v\n", connID, err)
    })
    
    // 设置消息处理回调
    wsServer.SetMessageHandler(func(connID string, msg types.Message) {
        fmt.Printf("收到来自 %s 的消息 [类型:%d]: %s\n", 
            connID, msg.Type, string(msg.Content))
        
        // 回应客户端
        responseMsg := types.Message{
            Type:    1,
            Content: []byte(fmt.Sprintf("已收到你的消息: %s", string(msg.Content))),
        }
        if err := wsServer.SendToClient(connID, responseMsg); err != nil {
            log.Printf("发送回应消息失败: %v", err)
        }
        
        // 广播消息给所有客户端
        if msg.Type == 1 { // 只广播文本消息
            broadcastMsg := types.Message{
                Type: 1,
                Content: []byte(fmt.Sprintf("用户 %s 说: %s", 
                    connID[:8], string(msg.Content))),
            }
            wsServer.BroadcastMessage(broadcastMsg)
        }
    })
    
    // 启动服务器
    go func() {
        if err := wsServer.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("服务器启动失败: %v", err)
        }
    }()
    
    // 处理信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    // 定时显示服务器状态
    statusTicker := time.NewTicker(10 * time.Second)
    defer statusTicker.Stop()
    
    for {
        select {
        case <-statusTicker.C:
            stats := wsServer.GetStats()
            fmt.Printf("服务器状态 - 客户端数: %d, 总消息: %d, 丢弃消息: %d\n",
                wsServer.GetClientCount(), stats.TotalMessages, stats.DroppedMessages)
        case sig := <-sigCh:
            fmt.Printf("收到信号: %v, 关闭服务器\n", sig)
            
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()
            
            if err := wsServer.Stop(ctx); err != nil {
                log.Printf("服务器关闭错误: %v", err)
            }
            return
        }
    }
}