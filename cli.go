package main

import (
	"log"

	"github.com/chanyipiaomiao/hltool"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (

	// rawPassword 明文密码
	rawPassword string

	// 以下是命令行参数设置
	cs            = kingpin.Command("cs", "c/s mode")
	mode          = cs.Flag("mode", "App Mode").Short('m').Default("client").String()
	file          = cs.Flag("filename", "Local File Path").Short('f').String()
	secret        = cs.Flag("secret", "Secret Key").Short('s').String()
	dest          = cs.Flag("dest", "Dest IP").Short('d').String()
	destPort      = cs.Flag("dest-port", "Dest IP SSH Port").Short('p').Default("9922").String()
	destUser      = cs.Flag("dest-user", "Dest IP User").Short('u').Default("wenba").String()
	remotePath    = cs.Flag("remote-path", "Remote File Path").Short('r').String()
	address       = cs.Flag("address", "Listen On Port, default: :6666").Short('i').String()
	peer          = cs.Flag("peer", "Peer Address, format: x.x.x.x:port").String()
	logRoot       = cs.Flag("log-root", "Log Root").Default("logs").String()
	sizeper       = cs.Flag("speed", "Transfer Speed").Default("4096").Int()
	enableEncrypt = cs.Flag("enable-encrypt", "Enable Encrypt Transfer").Short('e').Default("false").Bool()

	rsa       = kingpin.Command("rsa", "rsa manage")
	rsaKeyGen = rsa.Flag("keygen", "gen rsa key pair").Default("false").Short('k').Bool()
	bit       = rsa.Flag("bit", "key bits").Default("1024").Short('b').Int()
)

const (
	pubKeyName = "rsa.pub" // rsa 公钥名称
	priKeyName = "rsa.pri" // rsa 私钥名称
)

// ServerArgs 服务启动参数
type ServerArgs struct {
	Address string
	Secret  string
	LogRoot string
	TempDir string
}

// ClientArgs 客户端需要的参数
type ClientArgs struct {
	Filename        string // 文件名称
	Secret          string // 验证双方的密钥
	DestIP          string // 目标IP
	DestPort        string // 目标端口
	DestUser        string // 目标用户
	DestPath        string // 目标路径
	Peer            string // 对端IP和端口
	FileSize        int64  // 文件大小
	SizePer         int    // 每次传输多少个字节
	EnableEncrypt   bool   // 是否启用加密传输
	EncryptPassword []byte // 加密后的密码 传输到服务端 用于解密
}

// InitCli 初始化命令行参数
func InitCli() {
	kingpin.Version(AppVersion)
	c := kingpin.Parse()

	switch c {
	case "rsa":
		if *rsaKeyGen {
			err := hltool.NewRSAFile(pubKeyName, priKeyName, *bit)
			if err != nil {
				log.Fatalln(err)
			}
		}
	case "cs":
		switch *mode {
		case "server":
			serverArgs := &ServerArgs{
				Address: *address,
				LogRoot: *logRoot,
			}
			if *secret != "" {
				serverArgs.Secret = *secret
			}
			ServerListen(serverArgs)
		case "client":
			clientArgs := &ClientArgs{
				Filename:      *file,
				DestIP:        *dest,
				DestPort:      *destPort,
				DestUser:      *destUser,
				DestPath:      *remotePath,
				SizePer:       *sizeper,
				EnableEncrypt: *enableEncrypt,
			}
			if *secret != "" {
				clientArgs.Secret = *secret
			}
			if *peer != "" {
				clientArgs.Peer = *peer
			}
			if *enableEncrypt {
				rawPassword = hltool.GenRandomString(32, "no")
				gorsa, err := hltool.NewGoRSA(pubKeyName, priKeyName)
				if err != nil {
					log.Fatalln(err)
				}
				encrypt, err := gorsa.PublicEncrypt([]byte(rawPassword))
				if err != nil {
					log.Fatalln(err)
				}
				clientArgs.EncryptPassword = encrypt
			}
			ClientDial(clientArgs)
		default:
			log.Fatalln("run mode error")
		}
	default:
		log.Fatalln("sub commond error")
	}

}
