# MuTag 活码服务 · 多阶段构建（单二进制，最终镜像零 Go 工具链）
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mutag-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 mutag
COPY --from=build /out/mutag-server /usr/local/bin/mutag-server
USER mutag
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["mutag-server"]
