# 第一阶段：编译打包
FROM node:18-alpine AS builder
WORKDIR /app
RUN npm config set registry https://registry.npmmirror.com
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# 第二阶段：Nginx 托管
FROM nginx:alpine
WORKDIR /usr/share/nginx/html
# 拷贝打包产物
COPY --from=builder /app/dist .
# ✅ 拷贝自定义 Nginx 配置（包含 /api 代理）
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]