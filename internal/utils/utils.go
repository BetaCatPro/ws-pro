package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateMessageID 生成唯一消息ID
func GenerateMessageID() string {
    return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

// GenerateConnectionID 生成唯一连接ID
func GenerateConnectionID() string {
    return fmt.Sprintf("conn-%d", time.Now().UnixNano())
}

// GenerateRandomID 生成随机ID
func GenerateRandomID(length int) string {
    bytes := make([]byte, length/2+1)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)[:length]
}

// IsValidURL 检查URL是否有效
func IsValidURL(url string) bool {
    return len(url) > 0 && (url[:3] == "ws:" || url[:4] == "wss:")
}

// CalculateBackoff 计算退避时间
func CalculateBackoff(attempt int, baseTime time.Duration) time.Duration {
    if attempt <= 0 {
        return baseTime
    }
    return baseTime * time.Duration(1<<uint(attempt))
}