package ftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// FTPConfig FTP 配置结构体
type FTPConfig struct {
	Host     string // FTP服务器地址（如 192.168.1.100）
	Port     int    // FTP端口（默认21）
	User     string // 用户名
	Password string // 密码
	Timeout  int    // 连接超时时间（秒，默认5）
	// 新版库默认启用被动模式，无需手动配置
}

// FTPUtil FTP工具类
type FTPUtil struct {
	config FTPConfig
	conn   *ftp.ServerConn // FTP连接实例
}

// NewFTPUtil 初始化FTP工具类
func NewFTPUtil(config FTPConfig) *FTPUtil {
	// 设置默认值
	if config.Port == 0 {
		config.Port = 21
	}
	if config.Timeout == 0 {
		config.Timeout = 5
	}

	return &FTPUtil{
		config: config,
	}
}

// GetClient 获取FTP客户端连接（复用已有连接，断开则重连）
func (f *FTPUtil) GetClient() (*ftp.ServerConn, error) {
	// 检查现有连接是否有效
	if f.conn != nil {
		// 发送NOOP命令检测连接是否存活
		if err := f.conn.NoOp(); err == nil {
			return f.conn, nil
		}
		// 连接失效，关闭旧连接
		_ = f.conn.Quit()
		f.conn = nil
	}

	// 构建连接地址
	addr := fmt.Sprintf("%s:%d", f.config.Host, f.config.Port)

	// 正确的连接创建方式（新版库）
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(f.config.Timeout)*time.Second)
	defer cancel()

	// 组合配置项：上下文 + 超时
	conn, err := ftp.Dial(addr,
		ftp.DialWithContext(ctx),
		ftp.DialWithTimeout(time.Duration(f.config.Timeout)*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("FTP连接失败: %w", err)
	}

	// 登录
	if err := conn.Login(f.config.User, f.config.Password); err != nil {
		_ = conn.Quit()
		return nil, fmt.Errorf("FTP登录失败: %w", err)
	}

	// 🔥 新版库无需手动调用 EnterPassiveMode()，默认启用被动模式
	// 如需强制主动模式（极少场景），可使用：
	// conn.SetTransferMode(ftp.Active)

	f.conn = conn
	return conn, nil
}

// UploadImage 上传图片到FTP服务器
// remotePath: 远程文件路径（如 /uploads/2025/01/test.webp）
// imgBytes: 图片字节流
// contentType: 图片MIME类型（可选，仅日志用）
func (f *FTPUtil) UploadImage(remotePath string, imgBytes []byte, contentType string) error {
	// 获取客户端
	client, err := f.GetClient()
	if err != nil {
		return err
	}

	// 递归创建远程目录
	remoteDir := filepath.Dir(remotePath)
	if err := f.makeDirRecursive(client, remoteDir); err != nil {
		return fmt.Errorf("创建远程目录失败: %w", err)
	}

	// 上传文件
	reader := bytes.NewReader(imgBytes)
	if err := client.Stor(remotePath, reader); err != nil {
		return fmt.Errorf("上传图片失败: %w", err)
	}

	return nil
}

// GetFileStream 获取FTP文件流
// remotePath: 远程文件路径
// 返回值: 文件字节流、文件大小、错误
func (f *FTPUtil) GetFileStream(remotePath string) ([]byte, int64, error) {
	// 获取客户端
	client, err := f.GetClient()
	if err != nil {
		return nil, 0, err
	}

	// 获取文件读取流
	resp, err := client.Retr(remotePath)
	if err != nil {
		return nil, 0, fmt.Errorf("获取文件流失败: %w", err)
	}
	defer resp.Close()

	// 读取文件内容
	buf, err := io.ReadAll(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("读取文件流失败: %w", err)
	}

	return buf, int64(len(buf)), nil
}

// DeleteImage 删除FTP服务器上的图片
// remotePath: 远程文件路径
func (f *FTPUtil) DeleteImage(remotePath string) error {
	// 获取客户端
	client, err := f.GetClient()
	if err != nil {
		return err
	}

	// 删除文件
	if err := client.Delete(remotePath); err != nil {
		// 兼容不同FTP服务器的错误码（文件不存在）
		if strings.Contains(err.Error(), "550") || strings.Contains(err.Error(), "No such file") {
			return errors.New("文件不存在")
		}
		return fmt.Errorf("删除图片失败: %w", err)
	}

	return nil
}

// Close 关闭FTP连接
func (f *FTPUtil) Close() error {
	if f.conn != nil {
		err := f.conn.Quit()
		f.conn = nil
		return err
	}
	return nil
}

// makeDirRecursive 递归创建FTP目录
func (f *FTPUtil) makeDirRecursive(client *ftp.ServerConn, dir string) error {
	// 处理空目录/根目录
	if dir == "/" || dir == "." || dir == "" {
		return nil
	}

	// 修复：强制替换所有反斜杠为正斜杠
	dir = strings.ReplaceAll(dir, "\\", "/")
	// 拆分目录层级（仅按/拆分）
	dirs := strings.Split(strings.Trim(dir, "/"), "/")
	currentPath := ""

	for _, d := range dirs {
		if d == "" {
			continue
		}

		// 拼接当前层级（始终用/）
		if currentPath == "" {
			currentPath = d
		} else {
			currentPath = fmt.Sprintf("%s/%s", currentPath, d)
		}

		// 逐层级创建（关键：避免一次性创建多级）
		err := client.MakeDir(currentPath)
		// 兼容错误：目录已存在/权限提示
		if err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "550") || strings.Contains(errMsg, "already exists") {
				continue // 忽略已存在
			}
			if strings.Contains(errMsg, "553") {
				return fmt.Errorf("目录名被服务器禁止（%s）：%w", currentPath, err)
			}
			return fmt.Errorf("创建目录 %s 失败: %w", currentPath, err)
		}
	}

	return nil
}

// GetFileStreamReader 流式获取FTP文件
func (f *FTPUtil) GetFileStreamReader(remotePath string) (io.ReadCloser, int64, error) {
	client, err := f.GetClient()
	if err != nil {
		return nil, 0, err
	}

	// 获取文件读取流（Retr返回io.ReadCloser）
	resp, err := client.Retr(remotePath)
	if err != nil {
		return nil, 0, fmt.Errorf("获取文件流失败: %w", err)
	}

	var fileSize int64 = 0
	entries, err := client.List(remotePath)
	if err == nil && len(entries) > 0 && entries[0] != nil {
		// 安全转换：检查uint64是否超出int64范围
		if entries[0].Size <= uint64(math.MaxInt64) {
			fileSize = int64(entries[0].Size)
		} else {
			fileSize = 0
		}
	}

	return resp, fileSize, nil
}

// ListFiles 列出指定目录下的文件（可选扩展）
// remoteDir: 远程目录
// 返回值: 文件列表、错误
// func (f *FTPUtil) ListFiles(remoteDir string) ([]ftp.Entry, error) {
// 	client, err := f.GetClient()
// 	if err != nil {
// 		return nil, err
// 	}

// 	entries, err := client.List(remoteDir)
// 	if err != nil {
// 		return nil, fmt.Errorf("列出文件失败: %w", err)
// 	}

// 	return entries, nil
// }
