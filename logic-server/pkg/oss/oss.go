package oss

import (
	"fmt"
	"io"

	"github.com/Lhh220/g-video/logic-server/internal/config"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var Bucket *oss.Bucket

// InitOSS 初始化 OSS 客户端
func InitOSS() {
	c := config.GlobalConfig.OSS

	// 1. 创建客户端实例
	client, err := oss.New(c.Endpoint, c.AccessKeyID, c.AccessKeySecret)
	if err != nil {
		panic(fmt.Sprintf("OSS 客户端初始化失败: %v", err))
	}

	// 2. 获取 Bucket 对象
	Bucket, err = client.Bucket(c.BucketName)
	if err != nil {
		panic(fmt.Sprintf("获取 Bucket [%s] 失败: %v", c.BucketName, err))
	}

	// 3. 【核心探活步】尝试获取 Bucket 的基本信息（如元数据）
	// 如果 AccessKey 错误或权限不足，这一步会报错
	_, err = client.GetBucketInfo(c.BucketName)
	if err != nil {
		panic(fmt.Sprintf("OSS 认证失败或权限不足: %v", err))
	}

	fmt.Printf("✅ OSS 连接验证成功！Bucket: %s\n", c.BucketName)
}

// UploadFile 上传文件流到 OSS
// objectName: 存储在 OSS 上的路径和文件名 (例: "videos/user1/test.mp4")
// reader: 文件流
func UploadFile(objectName string, reader io.Reader) (string, error) {
	err := Bucket.PutObject(objectName, reader)
	if err != nil {
		return "", err
	}

	// 拼凑出文件的访问 URL
	// 格式: https://bucket-name.endpoint/objectName
	url := fmt.Sprintf("https://%s.%s/%s",
		config.GlobalConfig.OSS.BucketName,
		config.GlobalConfig.OSS.Endpoint,
		objectName)

	return url, nil
}
