package mq

import (
	"fmt"

	"github.com/streadway/amqp"
)

func PublishVideoMessage(videoID int64, authorID int64, videoURL string) error {
	msgBody := fmt.Sprintf(`{"video_id": %d, "author_id": %d, "url": "%s"}`, videoID, authorID, videoURL)

	return Channel.Publish(
		"video_publish", // exchange
		"",              // routing key
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(msgBody),
		},
	)
}
