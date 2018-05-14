package main

import (
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/chanyipiaomiao/hltool"
)

// ServerAckInfo 服务端响应的一些信息
type ServerAckInfo struct {
	RequestID    string
	DestIPStatus string
}

// IsPortOpen 检测端口是否已经打开状态
func IsPortOpen(ip, port string) (bool, error) {

	conn, err := net.DialTimeout("tcp", ip+":"+port, 2*time.Second)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	return true, nil
}

// GetLogger 返回Logger
func GetLogger(logRoot, filename string) *logs.BeeLogger {
	if !hltool.IsExist(logRoot) {
		os.MkdirAll(logRoot, os.ModePerm)
	}
	logger := logs.NewLogger()
	logger.SetLogger(logs.AdapterFile, fmt.Sprintf(`{"filename":"%s", "rotate": false}`, path.Join(logRoot, filename)))
	return logger
}

// StatFile 查看文件信息
func StatFile(filename string) (os.FileInfo, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return nil, fmt.Errorf("error: %s on os.Stat filename %s", err, filename)
	}
	return info, nil
}
