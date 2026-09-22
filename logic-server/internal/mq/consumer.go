package mq

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
	"github.com/Lhh220/g-video/logic-server/pkg/logx"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

// backoffs 处理失败后的重试间隔：200ms → 1s → 3s，共 4 次尝试
var backoffs = []time.Duration{200 * time.Millisecond, time.Second, 3 * time.Second}

// RunConsumers 启动视频发布事件的主消费者与死信消费者
func RunConsumers() {
	if !Enabled {
		return
	}

	// 手动 ack：重试成功才确认；重试耗尽 Nack(requeue=false) 进死信队列
	msgs, err := Channel.Consume(mainQueue, "", false, false, false, false, nil)
	if err != nil {
		logx.L().Error("启动主消费者失败", zap.Error(err))
		return
	}
	deadMsgs, err := Channel.Consume(deadQueue, "", false, false, false, false, nil)
	if err != nil {
		logx.L().Error("启动死信消费者失败", zap.Error(err))
		return
	}

	go consumeMain(msgs)
	go consumeDead(deadMsgs)

	logx.L().Info("✅ 视频发布事件消费者已启动 (queue: " + mainQueue + ", 死信: " + deadQueue + ")")
}

// consumeMain 主流程：解析 → 指数退避重试 → 成功 Ack / 耗尽 Nack 进死信
func consumeMain(msgs <-chan amqp.Delivery) {
	for d := range msgs {
		var msg VideoPublishMsg
		if err := json.Unmarshal(d.Body, &msg); err != nil {
			logx.L().Error("消息解析失败，转入死信", zap.Error(err), zap.ByteString("body", d.Body))
			_ = d.Nack(false, false)
			continue
		}

		var err error
		for attempt := 0; ; attempt++ {
			err = processVideoPublish(msg)
			if err == nil {
				break
			}
			if attempt >= len(backoffs) {
				break
			}
			logx.L().Warn("处理失败，准备重试",
				zap.Int64("video_id", msg.VideoID),
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoffs[attempt]),
				zap.Error(err))
			time.Sleep(backoffs[attempt])
		}

		if err != nil {
			logx.L().Error("重试耗尽，消息转入死信队列 (需人工介入)",
				zap.Int64("video_id", msg.VideoID), zap.Error(err))
			_ = d.Nack(false, false)
			continue
		}

		_ = d.Ack(false)
	}
}

// consumeDead 死信队列消费者：记录现场，等人工处理
func consumeDead(msgs <-chan amqp.Delivery) {
	for d := range msgs {
		deaths, _ := d.Headers["x-death"]
		logx.L().Error("☠️ 死信消息",
			zap.ByteString("body", d.Body),
			zap.Any("x_death", deaths))
		_ = d.Ack(false)
	}
}

// processVideoPublish 单条消息的处理逻辑，保持幂等：重复投递不会产生副作用
func processVideoPublish(msg VideoPublishMsg) error {
	// 1. 封面兜底：同步上传路径已生成封面，这里只补漏 (幂等：仅当为空时写)
	var v model.Video
	if err := database.DB.First(&v, msg.VideoID).Error; err == nil && v.CoverURL == "" {
		coverURL := msg.VideoURL + "?x-oss-process=video/snapshot,t_1000,f_jpg,w_0,h_0,m_fast"
		if err := database.DB.Model(&model.Video{}).
			Where("id = ?", msg.VideoID).
			Update("cover_url", coverURL).Error; err != nil {
			return fmt.Errorf("更新封面失败: %w", err)
		}
		logx.L().Info("封面兜底更新成功", zap.String("cover_url", coverURL))
	}

	// 2. 异步切片为 HLS：边下边播、弱网体验好 (无 ffmpeg 时自动跳过)
	runHLSTranscode(msg.VideoID, msg.VideoURL)

	// 3. 给粉丝发通知 (演示：真实项目查 follows 表写通知表)
	sendNotificationToFollowers(msg.AuthorID, msg.VideoID)

	return nil
}

func sendNotificationToFollowers(authorID int64, videoID int64) {
	// 伪代码示例：
	// 1. SELECT user_id FROM follows WHERE to_user_id = authorID
	// 2. FOR EACH follower: INSERT INTO messages (content, user_id) VALUES ("你关注的作者发布了新视频", follower)
	logx.L().Info("🔔 通知作者的粉丝", zap.Int64("author_id", authorID), zap.Int64("video_id", videoID))
}
