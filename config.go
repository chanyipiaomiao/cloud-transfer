package main

import (
	"log"

	"github.com/astaxie/beego/config"
)

var (

	// AppConfig 配置文件对象
	AppConfig config.Configer
)

func init() {
	var err error
	AppConfig, err = config.NewConfig("ini", "app.conf")
	if err != nil {
		log.Fatalf("读取配置文件错误: %s\n", err)
	}
}
