package service

import "testing"

// 对象 Key 由 MD5 决定：秒传去重与断点续传都依赖"同内容文件同 Key"这一约定
func TestObjectKeyForMD5(t *testing.T) {
	cases := []struct {
		md5      string
		filename string
		want     string
	}{
		{"d41d8cd98f00b204e9800998ecf8427e", "video.mp4", "videos/d41d8cd98f00b204e9800998ecf8427e.mp4"},
		{"abc123", "我的视频.MOV", "videos/abc123.MOV"},
		{"abc123", "noext", "videos/abc123"},
		{"abc123", "weird.name.mp4", "videos/abc123.mp4"},
	}

	for _, c := range cases {
		if got := objectKeyForMD5(c.md5, c.filename); got != c.want {
			t.Errorf("objectKeyForMD5(%q, %q) = %q, 期望 %q", c.md5, c.filename, got, c.want)
		}
	}
}
