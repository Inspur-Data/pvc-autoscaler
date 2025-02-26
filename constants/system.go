package constants

import (
	"fmt"
	"github.com/astaxie/beego"
	"strings"
)

const (
	/**
	用作代码调试
	*/
	CP_OPERATOR_RUN_MODE = "dev"
)

const (
	//运行模式
	RUN_MODE_DEV  = "dev"
	RUN_MODE_TEST = "test"
	RUN_MODE_PROD = "prod"
)

//运行模式
var RUN_MODE = beego.AppConfig.DefaultString("server.runmode", RUN_MODE_DEV)

//md5密钥
var Md5Salt = beego.AppConfig.DefaultString("system.md5-salt", "Dh@)!^o5l3!%Op0f")

//des密钥
var DESSalt = []byte(beego.AppConfig.DefaultString("system.des-salt", "incloudOS@inspur"))

//aes密钥
var AESSalt = []byte(beego.AppConfig.DefaultString("system.aes-salt", "incloudOS@inspur"))

//des/ecb密钥（兼容Java版）
var DESEcbKey = []rune{78, -7, -101, 25, 13, 18, 93, -12, -28, 21, -16, -33, 4, 105, -115, 2}

//aes/ecb密钥（兼容Java版）
var AESEcbKey = []rune{78, -7, -101, 25, 13, 18, 93, -12, -28, 21, -16, -33, 4, 105, -115, 2}

//日志文件根目录 默认:logs/
var LOGGER_ROOT = fmt.Sprintf("%s/", strings.TrimSuffix(beego.AppConfig.DefaultString("logger.path", "logs"), "/"))
