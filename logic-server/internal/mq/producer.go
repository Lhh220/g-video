package mq

import (
	"encoding/json"

	"github.com/streadway/amqp"
)

// VideoPublishMsg 视频发布事件，生产者与消费者共用
type VideoPublishMsg struct {
	VideoID  int64  `json:"video_id"`
	AuthorID int64  `json:"author_id"`
	VideoURL string `json:"url"`
}

// PublishVideoMessage 发布视频事件到 MQ，由消费者异步扩散 (转码/通知粉丝)
func PublishVideoMessage(videoID int64, authorID int64, videoURL string) error {
	if !Enabled {
		return nil // MQ 未启用时静默跳过
	}

	body, err := json.Marshal(VideoPublishMsg{
		VideoID:  videoID,
		AuthorID: authorID,
		VideoURL: videoURL,
	})
	if err != nil {
		return err
	}

	return Channel.Publish(
		"video_publish", // exchange
		"",              // routing key
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // 消息落盘，MQ 重启不丢
			Body:         body,
		},
	)
}
