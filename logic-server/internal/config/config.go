package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Database struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"database"`

	//  OSS 对应结构
	OSS struct {
		Endpoint        string `mapstructure:"endpoint"`
		AccessKeyID     string `mapstructure:"access_key_id"`
		AccessKeySecret string `mapstructure:"access_key_secret"`
		BucketName      string `mapstructure:"bucket_name"`
	} `mapstructure:"oss"`
	// Redis 对应结构
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`

	// 管理员引导账号：注册接口已禁止指定 role，
	// 管理员只能在服务启动时通过这里自动创建
	Admin struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"admin"`
}

var GlobalConfig Config

func InitConfig() {
	viper.SetConfigName("config")     // 配置文件名 (不带后缀)
	viper.SetConfigType("yaml")       // 配置文件类型
	viper.AddConfigPath("../configs") // 配置文件路径

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("配置解析失败: %v", err)
	}
}
