package mq

import (
	"fmt"

	"github.com/streadway/amqp"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

// Enabled 表示 MQ 是否可用；未配置或连接失败时为 false，
// 生产者静默跳过，主服务不受影响 (优雅降级)
var Enabled = false

// InitRabbitMQ 初始化 MQ 连接与交换机；任何一步失败都只降级、不 panic
func InitRabbitMQ(url string) {
	if url == "" {
		fmt.Println("⚠️ 未配置 rabbitmq.url，异步视频处理已禁用")
		return
	}

	var err error
	Conn, err = amqp.Dial(url)
	if err != nil {
		fmt.Printf("⚠️ 无法连接 RabbitMQ (%v)，异步视频处理已禁用\n", err)
		return
	}

	Channel, err = Conn.Channel()
	if err != nil {
		fmt.Printf("⚠️ 无法打开 RabbitMQ Channel (%v)，异步视频处理已禁用\n", err)
		return
	}

	// 声明 "video_publish" 交换机 (fanout: 一条消息广播给所有消费者)
	if err := Channel.ExchangeDeclare(
		"video_publish",
		"fanout",
		true,  // durable: 服务重启交换机不丢
		false, // auto-deleted
		false,
		false,
		nil,
	); err != nil {
		fmt.Printf("⚠️ 声明 video_publish 交换机失败 (%v)，异步视频处理已禁用\n", err)
		return
	}

	Enabled = true
	fmt.Println("✅ RabbitMQ 连接成功，video_publish 交换机就绪")
}
