package mq

import (
	"fmt"

	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

var Conn *amqp.Connection
var Channel *amqp.Channel

// Enabled 表示 MQ 是否可用；未配置或连接失败时为 false，
// 生产者静默跳过，主服务不受影响 (优雅降级)
var Enabled = false

const (
	mainExchange = "video_publish"
	mainQueue    = "video_process_queue"

	// 死信链路：主队列消息 Nack(requeue=false) 后路由到 DLX，落入死信队列等人工介入
	dlxExchange = "video_publish_dlx"
	deadQueue   = "video_process_dead_queue"
)

// declareTopology 声明全套拓扑：业务交换机 + 主队列(带死信路由) + 死信交换机/队列
func declareTopology() error {
	// 1. 业务交换机 (fanout: 一条消息广播给所有消费者)
	if err := Channel.ExchangeDeclare(mainExchange, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 %s 交换机失败: %w", mainExchange, err)
	}

	// 2. 死信交换机 + 死信队列
	if err := Channel.ExchangeDeclare(dlxExchange, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明死信交换机失败: %w", err)
	}
	dq, err := Channel.QueueDeclare(deadQueue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("声明死信队列失败: %w", err)
	}
	if err := Channel.QueueBind(dq.Name, "", dlxExchange, false, nil); err != nil {
		return fmt.Errorf("绑定死信队列失败: %w", err)
	}

	// 3. 主队列：处理失败的消息经 x-dead-letter-exchange 转入死信队列
	// 注意：队列参数不可变，若旧队列无此参数会声明失败，需删除旧队列后重启
	args := amqp.Table{"x-dead-letter-exchange": dlxExchange}
	if _, err := Channel.QueueDeclare(mainQueue, true, false, false, false, args); err != nil {
		return fmt.Errorf("声明主队列失败 (旧队列参数不一致时执行 rabbitmqctl delete_queue %s 后重启): %w", mainQueue, err)
	}
	if err := Channel.QueueBind(mainQueue, "", mainExchange, false, nil); err != nil {
		return fmt.Errorf("绑定主队列失败: %w", err)
	}
	return nil
}

// InitRabbitMQ 初始化 MQ 连接与拓扑；任何一步失败都只降级、不 panic
func InitRabbitMQ(url string) {
	if url == "" {
		logx.L().Warn("未配置 rabbitmq.url，异步视频处理已禁用")
		return
	}

	var err error
	Conn, err = amqp.Dial(url)
	if err != nil {
		logx.L().Warn("无法连接 RabbitMQ，异步视频处理已禁用", zap.Error(err))
		return
	}

	Channel, err = Conn.Channel()
	if err != nil {
		logx.L().Warn("无法打开 RabbitMQ Channel，异步视频处理已禁用", zap.Error(err))
		return
	}

	if err := declareTopology(); err != nil {
		logx.L().Warn("声明 MQ 拓扑失败，异步视频处理已禁用", zap.Error(err))
		return
	}

	Enabled = true
	logx.L().Info("✅ RabbitMQ 连接成功，video_publish 交换机就绪")
}

// Close 优雅关闭 MQ 连接 (退出前调用，尽量让在途消息处理完)
func Close() {
	if !Enabled {
		return
	}
	if Channel != nil {
		_ = Channel.Close()
	}
	if Conn != nil {
		_ = Conn.Close()
	}
	logx.L().Info("MQ 连接已关闭")
}
