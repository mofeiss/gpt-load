# 前端构建阶段
FROM node:20-alpine AS frontend-builder

# 更新系统并安装证书 - 跳过SSL验证
RUN apk --no-check-certificate update && \
    apk --no-check-certificate upgrade && \
    apk --no-check-certificate add --no-cache ca-certificates && \
    update-ca-certificates

ARG VERSION=1.0.0
WORKDIR /build

# 配置 npm，解决网络和证书问题
RUN npm config set registry https://registry.npmmirror.com/ && \
    npm config set strict-ssl false && \
    npm config set fetch-retry-maxtimeout 60000 && \
    npm config set fetch-retry-mintimeout 10000 && \
    npm config set fetch-timeout 300000

# 先只复制package.json和package-lock.json (如果存在)
COPY ./web/package.json ./web/package-lock.json* ./

# 安装依赖项 - 这一层只有在package.json改变时才会重新构建
RUN npm cache clean --force && \
    npm install --omit=optional --legacy-peer-deps --no-audit --no-fund || \
    (npm config set registry https://registry.npmjs.org/ && \
     npm install --omit=optional --legacy-peer-deps --no-audit --no-fund)

# 解决 rollup 原生模块问题
RUN npm config set target_arch x64 && \
    npm config set target_platform linux && \
    npm install @rollup/rollup-linux-arm64-gnu --save-optional || \
    npm install @rollup/rollup-linux-x64-gnu --save-optional || true

# 设置环境变量，强制使用 JavaScript 实现
ENV ROLLUP_NO_NATIVE=1

# 现在复制源代码 - 只有源代码改变时才重新构建这一层
COPY ./web .

# 构建项目
RUN VITE_VERSION=${VERSION} npm run build


# Go后端构建阶段
FROM golang:alpine AS backend-builder

# 安装证书和配置代理
RUN apk --no-check-certificate update && \
    apk --no-check-certificate add --no-cache ca-certificates git && \
    update-ca-certificates

ARG VERSION=1.0.0
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct \
    GOSUMDB=sum.golang.google.cn

WORKDIR /build

# 先复制go.mod和go.sum - 依赖只有在这些文件改变时才重新下载
COPY go.mod go.sum ./
RUN go mod download

# 复制Go源代码
COPY . .

# 复制前端构建结果
COPY --from=frontend-builder /build/dist ./web/dist

# 构建Go应用
RUN go build -ldflags "-s -w -X gpt-load/internal/version.Version=${VERSION}" -o gpt-load


# 最终运行镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖 - 先安装证书包，跳过SSL验证
RUN apk --no-check-certificate update && \
    apk --no-check-certificate upgrade --no-cache && \
    apk --no-check-certificate add --no-cache ca-certificates tzdata && \
    update-ca-certificates

# 复制构建好的应用
COPY --from=backend-builder /build/gpt-load .

EXPOSE 3001
ENTRYPOINT ["/app/gpt-load"]
