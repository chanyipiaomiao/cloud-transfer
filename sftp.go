package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/chanyipiaomiao/hltool"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig ssh连接配置
type SSHConfig struct {
	Host     string
	Username string
	Port     string
	Timeout  time.Duration
}

// GetKeyFile 得到用户的私钥
func GetKeyFile(username string) (ssh.AuthMethod, error) {
	home, err := hltool.UserHome()
	if err != nil {
		return nil, fmt.Errorf("Get hltool.UserHome() error: %s", err)
	}
	if "" == home {
		if username == "root" {
			home = "/root"
		} else {
			home = "/home/" + username
		}
	}
	privateKey := home + "/.ssh/id_rsa"
	buffer, err := ioutil.ReadFile(privateKey)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadFile(privateKey) error: %s", err)
	}
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("ssh.ParsePrivateKey(buffer) error: %s", err)
	}
	return ssh.PublicKeys(key), nil
}

// SFTP SFTP客户端
type SFTP struct {
	SSHConfig  *SSHConfig
	SFTPClient *sftp.Client
	RequestID  string
}

// Connect SFTP客户端
func (s *SFTP) Connect() error {
	privateKey, err := GetKeyFile(s.SSHConfig.Username)
	if err != nil {
		return err
	}

	clientConfig := &ssh.ClientConfig{
		User: s.SSHConfig.Username,
		Auth: []ssh.AuthMethod{
			privateKey,
		},
		Timeout: s.SSHConfig.Timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	sshClient, err := ssh.Dial("tcp", s.SSHConfig.Host+":"+s.SSHConfig.Port, clientConfig)
	if err != nil {
		return fmt.Errorf("ssh.Dial error: %s", err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("sftp.NewClient error: %s", err)
	}
	s.SFTPClient = sftpClient

	return nil
}

// Transfer SFTP传输文件
func (s *SFTP) Transfer(localPath, remotePath string) error {

	err := s.Connect()
	if err != nil {
		return err
	}

	// 打开本地文件
	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("os.Open(localPath) error: %s", err)
	}
	defer src.Close()

	// 创建远程文件
	dst, err := s.SFTPClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("s.Client.Create(remotePath) error: %s", err)
	}
	defer dst.Close()

	// 循环读取文件 然后写入文件
	buf := make([]byte, 8192)
	for {
		n, _ := src.Read(buf)
		if n == 0 {
			break
		}
		n, err = dst.Write(buf[:n])
		if err != nil {
			return fmt.Errorf("dst.Write(buf[:n]) error: %s", err)
		}
	}
	return nil
}
