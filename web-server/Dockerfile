FROM golang:1.25 AS builder
RUN apt update && apt install -y --no-install-recommends git gcc libc6-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd /app/web-server/cmd && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/web_srv ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/web_srv .


EXPOSE 8080
CMD ["./web_srv"]