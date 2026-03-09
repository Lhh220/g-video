# 定义变量，方便以后修改
PROTO_DIR=api/proto
OUT_DIR=.

.PHONY: proto
# 一键生成所有 proto 代码
proto:
	protoc --go_out=$(OUT_DIR) --go-grpc_out=$(OUT_DIR) $(PROTO_DIR)/user.proto
	protoc --go_out=$(OUT_DIR) --go-grpc_out=$(OUT_DIR) $(PROTO_DIR)/video.proto
	protoc --go_out=$(OUT_DIR) --go-grpc_out=$(OUT_DIR) $(PROTO_DIR)/social.proto

.PHONY: build
# 一键编译两个服务
build:
	go build -o bin/web-server web-server/cmd/main.go
	go build -o bin/logic-server logic-server/cmd/main.go

.PHONY: clean
# 清理生成的二进制文件
clean:
	rm -rf bin/