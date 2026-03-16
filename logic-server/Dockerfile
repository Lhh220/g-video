# 第一阶段：编译
FROM golang:1.25 AS builder
RUN apt update && apt install -y --no-install-recommends git gcc libc6-dev && rm -rf /var/lib/apt/lists/*
ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /app
# 设置代理加速下载
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 编译二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/logic_srv ./logic-server/cmd/main.go

# 第二阶段：运行
FROM alpine:latest
WORKDIR /app
# 拷贝二进制文件
COPY --from=builder /app/logic_srv .

# ✅ 核心修改：把配置文件直接拷贝到 /configs（代码里找的就是这个目录）
COPY --from=builder /app/logic-server/configs /configs

EXPOSE 50051
CMD ["./logic_srv"]