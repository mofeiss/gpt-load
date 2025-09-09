FROM node:20-alpine AS builder

# 更新系统并安装证书
RUN apk update && apk upgrade && \
    apk add --no-cache ca-certificates && \
    update-ca-certificates

ARG VERSION=1.0.0
WORKDIR /build
COPY ./web .

# 配置 npm，解决网络和证书问题
RUN npm config set registry https://registry.npmmirror.com/ && \
    npm config set strict-ssl false && \
    npm config set fetch-retry-maxtimeout 60000 && \
    npm config set fetch-retry-mintimeout 10000 && \
    npm config set fetch-timeout 300000 && \
    rm -rf node_modules package-lock.json && \
    npm cache clean --force

# 安装依赖项，跳过可选依赖并设置更宽松的策略
RUN npm install --omit=optional --legacy-peer-deps --no-audit --no-fund || \
    (npm config set registry https://registry.npmjs.org/ && \
     npm install --omit=optional --legacy-peer-deps --no-audit --no-fund)

# 解决 rollup 原生模块问题
RUN npm config set target_arch x64 && \
    npm config set target_platform linux && \
    npm install @rollup/rollup-linux-arm64-gnu --save-optional || \
    npm install @rollup/rollup-linux-x64-gnu --save-optional || true

# 设置环境变量，强制使用 JavaScript 实现
ENV ROLLUP_NO_NATIVE=1

# 构建项目
RUN VITE_VERSION=${VERSION} npm run build


FROM golang:alpine AS builder2

ARG VERSION=1.0.0
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X gpt-load/internal/version.Version=${VERSION}" -o gpt-load


FROM alpine

WORKDIR /app
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates

COPY --from=builder2 /build/gpt-load .
EXPOSE 3001
ENTRYPOINT ["/app/gpt-load"]
