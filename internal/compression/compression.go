package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/snappy"
)

// Compressor 压缩器接口
type Compressor interface {
    Compress([]byte) ([]byte, error)   // 压缩数据
    Decompress([]byte) ([]byte, error) // 解压缩数据
}

// GzipCompressor Gzip压缩实现
type GzipCompressor struct{}

// Compress 使用Gzip压缩数据
func (g *GzipCompressor) Compress(data []byte) ([]byte, error) {
    var buf bytes.Buffer
    w := gzip.NewWriter(&buf)
    if _, err := w.Write(data); err != nil {
        return nil, err
    }
    if err := w.Close(); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

// Decompress 使用Gzip解压缩数据（添加安全检查）
func (g *GzipCompressor) Decompress(data []byte) ([]byte, error) {
    // 安全检查：空数据或过短的数据
    if len(data) < 2 {
        return nil, fmt.Errorf("data too short to be gzip compressed")
    }
    
    // 检查gzip魔数（1F 8B）
    if data[0] != 0x1F || data[1] != 0x8B {
        return nil, fmt.Errorf("invalid gzip header")
    }
    
    r, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    defer r.Close()
    return io.ReadAll(r)
}

// SnappyCompressor Snappy压缩实现
type SnappyCompressor struct{}

// Compress 使用Snappy压缩数据
func (s *SnappyCompressor) Compress(data []byte) ([]byte, error) {
    return snappy.Encode(nil, data), nil
}

// Decompress 使用Snappy解压缩数据（添加安全检查）
func (s *SnappyCompressor) Decompress(data []byte) ([]byte, error) {
    // 安全检查
    if len(data) == 0 {
        return nil, fmt.Errorf("empty data")
    }
    
    decoded, err := snappy.Decode(nil, data)
    if err != nil {
        return nil, fmt.Errorf("snappy decompress failed: %v", err)
    }
    return decoded, nil
}

// GetCompressor 根据名称获取压缩器
func GetCompressor(name string) Compressor {
    switch name {
    case "snappy":
        return &SnappyCompressor{}
    case "gzip":
    default:
        return &GzipCompressor{}
    }
    return &GzipCompressor{}
}