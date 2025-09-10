# 前端构建阶段
FROM node:20-alpine AS frontend
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 后端构建阶段  
FROM golang:1.23-alpine AS backend
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o gpt-load .

# 运行阶段 - 使用极简镜像
FROM scratch
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /app/gpt-load /gpt-load
EXPOSE 3001
ENTRYPOINT ["/gpt-load"]
