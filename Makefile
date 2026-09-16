# 定义变量，方便以后修改
PROTO_DIR=api/proto
OUT_DIR=.
MODULE=github.com/Lhh220/g-video

.PHONY: proto
# 一键生成所有 proto 代码
# 注意：必须带 -I. 才能解析 video.proto 里 import 的 user.proto；
# user.proto 的 go_package 是完整模块路径，需要 module 选项剥离前缀，否则生成到错误目录
proto:
	protoc -I. --go_out=$(OUT_DIR) --go_opt=module=$(MODULE) --go-grpc_out=$(OUT_DIR) --go-grpc_opt=module=$(MODULE) $(PROTO_DIR)/user.proto
	protoc -I. --go_out=$(OUT_DIR) --go-grpc_out=$(OUT_DIR) $(PROTO_DIR)/video.proto
	protoc -I. --go_out=$(OUT_DIR) --go-grpc_out=$(OUT_DIR) $(PROTO_DIR)/social.proto

.PHONY: build
# 一键编译两个服务
build:
	go build -o bin/web-server web-server/cmd/main.go
	go build -o bin/logic-server logic-server/cmd/main.go

.PHONY: clean
# 清理生成的二进制文件
clean:
	rm -rf bin/