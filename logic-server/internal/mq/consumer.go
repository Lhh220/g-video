package mq

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
)

// RunConsumers 启动视频发布事件的异步消费者
func RunConsumers() {
	if !Enabled {
		return
	}

	// 1. 声明持久化队列并绑定到交换机
	q, err := Channel.QueueDeclare("video_process_queue", true, false, false, false, nil)
	if err != nil {
		log.Printf("声明队列失败: %v", err)
		return
	}
	if err := Channel.QueueBind(q.Name, "", "video_publish", false, nil); err != nil {
		log.Printf("绑定队列失败: %v", err)
		return
	}

	// 2. 手动 ack：处理成功才确认，失败不重投 (避免坏消息无限循环)，靠日志人工介入
	msgs, err := Channel.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Printf("启动消费者失败: %v", err)
		return
	}

	go func() {
		for d := range msgs {
			var msg VideoPublishMsg
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("解析消息失败: %v", err)
				_ = d.Nack(false, false)
				continue
			}

			log.Printf("🚀 开始处理视频扩散: VideoID=%d", msg.VideoID)

			if err := processVideoPublish(msg); err != nil {
				log.Printf("处理视频扩散失败 (不重投): %v", err)
				_ = d.Nack(false, false)
				continue
			}

			_ = d.Ack(false)
		}
	}()

	fmt.Println("✅ 视频发布事件消费者已启动 (queue: video_process_queue)")
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
		log.Printf("✅ 封面兜底更新成功: %s", coverURL)
	}

	// 2. 给粉丝发通知 (演示：真实项目查 follows 表写通知表)
	sendNotificationToFollowers(msg.AuthorID, msg.VideoID)

	return nil
}

func sendNotificationToFollowers(authorID int64, videoID int64) {
	// 伪代码示例：
	// 1. SELECT user_id FROM follows WHERE to_user_id = authorID
	// 2. FOR EACH follower: INSERT INTO messages (content, user_id) VALUES ("你关注的作者发布了新视频", follower)
	log.Printf("🔔 正在通知作者 %d 的粉丝，新视频 ID: %d", authorID, videoID)
}
