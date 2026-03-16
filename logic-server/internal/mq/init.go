package mq

import (
	"log"

	"github.com/streadway/amqp"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

func InitRabbitMQ(url string) {
	var err error
	Conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatalf("无法连接 RabbitMQ: %v", err)
	}

	Channel, err = Conn.Channel()
	if err != nil {
		log.Fatalf("无法打开 Channel: %v", err)
	}

	// 声明一个名为 "video_publish" 的交换机
	err = Channel.ExchangeDeclare(
		"video_publish", // name
		"fanout",        // type (扇出模式：一个消息发给多个消费者)
		true,            // durable
		false,           // auto-deleted
		false,
		false,
		nil,
	)
}
