package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BetaCatPro/ws-pro/pkg/client"
	"github.com/BetaCatPro/ws-pro/pkg/types"
	"github.com/gorilla/websocket"
)

func main() {
    config := &types.Config{
        HeartbeatInterval: 5 * time.Second,
        HeartbeatMsg:      "ping",
        ReconnectBaseTime: 1 * time.Second,
        MaxReconnectTimes: 6,
        BufferSize:        1000,
        MaxConcurrent:     10,
    }
    
    // 创建客户端
    wsClient := client.NewClient("wss://echo.websocket.org", config)
    
    // 设置消息处理回调
    wsClient.SetMessageHandler(func(msg types.Message) {
        fmt.Printf("收到消息 [类型:%d, ID:%s]: %s\n", 
            msg.Type, msg.ID, string(msg.Content))
        
        // 这里可以处理不同类型的消息
        switch msg.Type {
        case websocket.TextMessage: // TextMessage
            fmt.Printf("文本消息: %s\n", string(msg.Content))
        case websocket.BinaryMessage: // BinaryMessage
            fmt.Printf("二进制消息长度: %d bytes\n", len(msg.Content))
        default:
            // 忽略控制消息
            fmt.Printf("收到控制消息，类型: %d\n", msg.Type)
        }
    })
    
    // 设置连接成功回调
    wsClient.SetConnectHandler(func() {
        fmt.Println("连接服务器成功！")
        
        // 连接成功后发送测试消息
        go func() {
            time.Sleep(1 * time.Second)
            testMsg := types.Message{
                Type:    1, // TextMessage
                Content: []byte("Hello WebSocket Server!"),
            }
            if err := wsClient.SendMessage(testMsg); err != nil {
                log.Printf("发送消息失败: %v", err)
            }
        }()
    })
    
    // 设置断开连接回调
    wsClient.SetDisconnectHandler(func(err error) {
        fmt.Printf("连接断开: %v\n", err)
    })
    
    // 设置自定义请求头
    // wsClient.SetHeader("Authorization", "Bearer token")
    // wsClient.SetHeader("User-Agent", "ws-pro-client/1.0.0")
    
    // 连接服务器
    if err := wsClient.Connect(); err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer wsClient.Close()
    
    // 处理信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    // 定时发送消息
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if wsClient.IsConnected() {
                msg := types.Message{
                    Type:    1,
                    Content: []byte(fmt.Sprintf("当前时间: %v", time.Now())),
                }
                if err := wsClient.SendMessage(msg); err != nil {
                    log.Printf("发送消息失败: %v", err)
                }
            }
        case sig := <-sigCh:
            fmt.Printf("收到信号: %v, 退出程序\n", sig)
            return
        }
    }
}