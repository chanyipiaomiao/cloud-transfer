package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/chanyipiaomiao/hltool"
	pb "gopkg.in/cheggaaa/pb.v1"
)

// ClientDial 客户端连接服务器
func ClientDial(clientArgs *ClientArgs) {

	// 1. 连接服务端
	var address string
	if clientArgs.Peer != "" {
		address = clientArgs.Peer
	} else {
		address = AppConfig.String("client::peer")
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Fatalf("client error: %s on net.Dial\n", err)
	}
	defer conn.Close()

	// 2. 读取文件大小
	info, err := StatFile(clientArgs.Filename)
	if err != nil {
		log.Fatalf("client error: %s on StatFile(clientArgs.Filename)\n", err)
	}
	clientArgs.FileSize = info.Size()

	// 3. 发送密钥 目标IP、目标路径文件名、文件大小, 是否启用加密传输及加密传输的密码
	if clientArgs.Secret == "" {
		clientArgs.Secret = AppConfig.String("security::secret")
	}
	s, err := hltool.StructToBytes(clientArgs)
	if err != nil {
		log.Fatalf("client error: %s on StructToBytes(clientArgs)\n", err)
	}
	_, err = conn.Write(s)
	if err != nil {
		log.Fatalf("client error: %s on Write clientArgs\n", err)
	}

	// 4. 接收RequestId 和 接收 目标IP和端口 OK结果
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("client error: %s on Receive RequestId\n", err)
	}
	serverAck := &ServerAckInfo{}
	err = hltool.BytesToStruct(buf[:n], serverAck)
	if err != nil {
		log.Fatalf("client error: %s on bytes to ServerAckInfo struct\n", err)
	}
	log.Printf("[RequestId]: %s\n", serverAck.RequestID)
	log.Printf("[数据流向]: 客户端 ----------> 中转服务器 ----------> 目标服务器\n")

	if "DestIP Connect OK" != serverAck.DestIPStatus {
		log.Fatalf("client error on Receive (DestIP Connect Status), Expect: DestIP Connect OK, Got: %s\n", serverAck.DestIPStatus)
	}

	// 5. 发送文件内容
	log.Println("[开始]发送文件到中转服务器")
	err = SendFile(conn, clientArgs)
	if err != nil {
		log.Fatalf("%s\n", err)
	}
	log.Println("[完成]发送文件到中转服务器")
	log.Println("[等待]中转服务器传输文件到目标服务器")

	// 6. 接收 文件已经传输到目标机器通知
	n, err = conn.Read(buf)
	if err != nil {
		log.Fatalf("client error: %s on conn.Read (File Transfered Remote)\n", err)
	}

	if "File Already Transfered Remote" != string(buf[:n]) {
		log.Fatalf("client error on Receive (File Already Transfered Remote), Expect: File Already Transfered Remote, Got: %s\n", string(buf[:n]))
	}
	log.Println("[完成]中转服务器传输文件到目标服务器")
}

// SendFile 发送文件
func SendFile(conn net.Conn, clientArgs *ClientArgs) error {
	f, err := os.Open(clientArgs.Filename)
	if err != nil {
		return fmt.Errorf("client error: %s on os.Open", err)
	}
	defer f.Close()

	var goaes *hltool.GoAES
	if clientArgs.EnableEncrypt {
		log.Println("[加密]启用了加密传输")
		goaes = hltool.NewGoAES([]byte(rawPassword))
	}

	// 设置进度条
	bar := pb.New64(clientArgs.FileSize).SetUnits(pb.U_BYTES)
	bar.Start()
	bar.ShowTimeLeft = true
	bar.ShowSpeed = true

	// 读取文件 发送文件
	buf := make([]byte, clientArgs.SizePer)
	for {
		n, err := f.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("client error: %s on f.Read", err)
			}
		}

		var data []byte
		if clientArgs.EnableEncrypt {
			var err error
			data, err = goaes.Encrypt(buf[:n])
			if err != nil {
				return fmt.Errorf("client error: %s on SendFile goaes.Encrypt(buf[:n])", err)
			}
		} else {
			data = buf[:n]
		}
		n, err = conn.Write(data)
		if err != nil {
			return fmt.Errorf("client error: %s on SendFile conn.Write", err)
		}
		bar.Add(n)

	}
	bar.Finish()
	return nil
}
