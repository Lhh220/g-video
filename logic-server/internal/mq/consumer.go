package mq

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Lhh220/g-video/logic-server/internal/model"
	"github.com/Lhh220/g-video/logic-server/pkg/database"
)

// 定义消息结构体，保持和生产者一致
type VideoPublishMsg struct {
	VideoID  int64  `json:"video_id"`
	AuthorID int64  `json:"author_id"`
	VideoURL string `json:"url"`
}

func RunConsumers() {
	// 1. 声明队列并绑定
	q, _ := Channel.QueueDeclare("video_process_queue", true, false, false, false, nil)
	Channel.QueueBind(q.Name, "", "video_publish", false, nil)

	msgs, _ := Channel.Consume(q.Name, "", true, false, false, false, nil)

	go func() {
		for d := range msgs {
			var msg VideoPublishMsg
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("解析消息失败: %v", err)
				continue
			}

			log.Printf("🚀 开始处理视频扩散: VideoID=%d", msg.VideoID)

			// --- 1. 调用 OSS 截帧策略 ---
			// 阿里云 OSS 格式：视频URL?x-oss-process=video/snapshot,t_1000,f_jpg,w_800,h_600,m_fast
			// 这里的 t_1000 表示截取第 1000 毫秒（第1秒）的画面
			coverURL := fmt.Sprintf("%s?x-oss-process=video/snapshot,t_1000,f_jpg", msg.VideoURL)

			// --- 2. 更新数据库里的 cover_url ---
			err := database.DB.Model(&model.Video{}).
				Where("id = ?", msg.VideoID).
				Update("cover_url", coverURL).Error
			if err != nil {
				log.Printf("更新封面失败: %v", err)
			} else {
				log.Printf("✅ 封面更新成功: %s", coverURL)
			}

			// --- 3. 给粉丝发通知 (这里演示逻辑) ---
			// 实际项目中，你可以查关注表找到该作者的所有粉丝，然后往消息通知表插数据
			sendNotificationToFollowers(msg.AuthorID, msg.VideoID)
		}
	}()
}

func sendNotificationToFollowers(authorID int64, videoID int64) {
	// 伪代码示例：
	// 1. SELECT user_id FROM follows WHERE to_user_id = authorID
	// 2. FOR EACH follower: INSERT INTO messages (content, user_id) VALUES ("你关注的作者发布了新视频", follower)
	log.Printf("🔔 正在通知作者 %d 的粉丝，新视频 ID: %d", authorID, videoID)
}
