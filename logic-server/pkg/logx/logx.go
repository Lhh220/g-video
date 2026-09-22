package logx

import (
	"go.uber.org/zap"
)

// 全局 zap 日志器；未 Init 时为空实现，保证不初始化也能安全调用
var logger = zap.NewNop()

// Init 初始化全局日志器。dev=true 输出人类可读的控制台格式，否则 JSON 结构化输出
func Init(dev bool) {
	var (
		l   *zap.Logger
		err error
	)
	if dev {
		l, err = zap.NewDevelopment()
	} else {
		l, err = zap.NewProduction()
	}
	if err != nil {
		panic("初始化日志失败: " + err.Error())
	}
	logger = l
}

// L 返回全局日志器
func L() *zap.Logger { return logger }
