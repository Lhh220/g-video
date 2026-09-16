package config

import (
	"log"
	"os"

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

	// RabbitMQ：未配置或连不上时自动降级，不影响主服务
	RabbitMQ struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"rabbitmq"`

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

	// 环境变量优先于配置文件 (Docker 部署注入，本地开发留空即用 yaml)
	// 注意：viper.AutomaticEnv 对 struct Unmarshal 不生效，这里手动覆盖
	if v := os.Getenv("DB_DSN"); v != "" {
		GlobalConfig.Database.DSN = v
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		GlobalConfig.Redis.Addr = v
	}
	if v := os.Getenv("MQ_URL"); v != "" {
		GlobalConfig.RabbitMQ.URL = v
	}
}
