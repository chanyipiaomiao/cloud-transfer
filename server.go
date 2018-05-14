package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/chanyipiaomiao/hltool"
	"github.com/satori/go.uuid"
)

// ServerListen 服务端监听
func ServerListen(serverArgs *ServerArgs) {

	// 接收文件的临时目录 是否存在
	tempDir := AppConfig.String("server::tempDir")
	if !hltool.IsExist(tempDir) {
		os.MkdirAll(tempDir, os.ModePerm)
	}
	serverArgs.TempDir = tempDir

	var address string
	if serverArgs.Address != "" {
		address = serverArgs.Address
	} else {
		address = AppConfig.String("server::listen")
	}
	// 监听地址
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("server error: %s on Listen Port\n", err)
	}
	log.Printf("Listen On %s\n", address)

	// 处理多个请求
	for {
		// 接受请求
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("server error: %s on Listener.Accept\n", err)
			continue
		}
		// 处理请求
		go handConn(serverArgs, conn)
	}

}

// recordLog 返回logger
func recordLog(logRoot, logRequestID string) *logs.BeeLogger {
	return GetLogger(logRoot, logRequestID+".log")
}

// handConn 处理连接请求
func handConn(serverArgs *ServerArgs, conn net.Conn) {

	requestID, _ := uuid.NewV4()
	logRequestID := fmt.Sprintf("%s", requestID)
	logger := recordLog(serverArgs.LogRoot, logRequestID)

	// 发生 panic，错误会被记录且协程会退出并释放，而其他协程不受影响
	defer func() {
		conn.Close()
		if err := recover(); err != nil {
			logger.Error(fmt.Sprintf("Work failed with: %s", err))
		}
		logger.Close()
	}()

	logger.Info(fmt.Sprintf("client %s is connected", conn.RemoteAddr()))

	// 1. 接收密钥  目标IP、目标路径文件名
	// 判断密钥是否正确 不正确则返回错误并断开连接 密钥既可以从命令行读取也可以在配置文件中配置
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		panic(fmt.Sprintf("server conn.Read error: %s on Read ClientArgs", err))
	}
	destInfo := new(ClientArgs)
	err = hltool.BytesToStruct(buf[:n], destInfo)
	if err != nil {
		panic(fmt.Sprintf("server error: %s on bytes to ClientArgs struct: ", err))
	}

	if serverArgs.Secret == "" {
		if AppConfig.String("security::secret") != destInfo.Secret {
			panic("secret(from conf file) mismatch")
		}
	} else {
		if serverArgs.Secret != destInfo.Secret {
			panic("secret(from cli) mismatch")
		}
	}
	log.Printf("接受: %s 请求, 分配的RequestId: %s\n", conn.RemoteAddr(), logRequestID)

	// 2. 发送 RequestId 到客户端 对第1步的目标IP进行连通性判断,可以连接 返回ok
	ackInfo := &ServerAckInfo{
		RequestID: logRequestID,
	}

	for i := 0; i < 3; i++ {
		if ok, err := IsPortOpen(destInfo.DestIP, destInfo.DestPort); !ok {
			_, err1 := conn.Write([]byte("DestIP Connect Not OK"))
			if err1 != nil {
				panic(fmt.Sprintf("server conn.Write error: %s on write: DestIP Connect Not OK", err1))
			}
			panic(fmt.Sprintf("server error: %s on check dest ip(%s) port(%s) is alive: ", err, destInfo.DestIP, destInfo.DestPort))
		} else {
			ackInfo.DestIPStatus = "DestIP Connect OK"
			break
		}
	}

	s, err := hltool.StructToBytes(ackInfo)
	if err != nil {
		panic(fmt.Sprintf("server error: %s on StructToBytes(ackInfo)\n", err))
	}
	_, err = conn.Write(s)
	if err != nil {
		panic(fmt.Sprintf("server conn.Write error: %s on write: DestIP Connect OK", err))
	}
	logger.Info("发送RequestId(%s)到客户端", logRequestID)
	logger.Info(fmt.Sprintf("目标IP(%s) 端口: %s 连接OK", destInfo.DestIP, destInfo.DestPort))

	// 3. 接收文件内容
	filename := path.Base(destInfo.Filename)
	tempFile := path.Join(serverArgs.TempDir, filename)
	file, err := os.Create(tempFile)
	if err != nil {
		panic(fmt.Sprintf("server error: %s on Create File: %s", err, tempFile))
	}

	// 客户端是否启用了加密
	var goaes *hltool.GoAES
	if destInfo.EnableEncrypt {
		gorsa, err := hltool.NewGoRSA(pubKeyName, priKeyName)
		if err != nil {
			panic(fmt.Sprintf("gorsa error: %s", err))
		}
		decryptPassword, err := gorsa.PrivateDecrypt(destInfo.EncryptPassword)
		if err != nil {
			panic(fmt.Sprintf("server error on gorsa.PrivateDecrypt: %s", err))
		}
		goaes = hltool.NewGoAES(decryptPassword)
		logger.Info("本次操作启用了加密传输")
	}

	logger.Info("开始接收文件: %s", filename)
	buf = make([]byte, destInfo.SizePer+16)
	size := 0
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(fmt.Sprintf("server conn.Read error: %s on Read File Content", err))
			}
		}
		var data []byte
		if destInfo.EnableEncrypt {
			var err error
			data, err = goaes.Decrypt(buf[:n])
			if err != nil {
				panic(fmt.Sprintf("server error: %s on goaes.Decrypt(buf[:n])", err))
			}
		} else {
			data = buf[:n]
		}
		file.Write(data)
		size += len(data)
		if int64(size) == destInfo.FileSize {
			break
		}
	}
	file.Close()
	receiveInfo, err := StatFile(tempFile)
	if err != nil {
		panic(err)
	}
	if receiveInfo.Size() != destInfo.FileSize {
		panic(fmt.Sprintf("server conn.Read error on Read File Content, not receive complete, alreay receive: %d", receiveInfo.Size()))
	}
	logger.Info(fmt.Sprintf("接收文件结束,存储在: %s", tempFile))

	// 4. 传输文件到目标机器
	logger.Info(fmt.Sprintf("开始传输文件到目标机器: %s, 路径: %s", destInfo.DestIP, destInfo.DestPath))
	sftp := &SFTP{
		SSHConfig: &SSHConfig{
			Host:     destInfo.DestIP,
			Username: destInfo.DestUser,
			Port:     destInfo.DestPort,
			Timeout:  2 * time.Second,
		},
		RequestID: logRequestID,
	}
	err = sftp.Transfer(tempFile, path.Join(destInfo.DestPath, filename))
	if err != nil {
		panic(fmt.Sprintf("sftp.Transfer(tempFile, destInfo.DestPath) error: %s", err))
	}

	logger.Info("结束传输文件到目标机器")

	// 5. 回复文件已经传输到目标机器
	_, err = conn.Write([]byte("File Already Transfered Remote"))
	if err != nil {
		panic(fmt.Sprintf("server conn.Write (File Transfered Remote) error: %s", err))
	}
	logger.Info("通知客户端文件已经传输到目标服务器")

}
