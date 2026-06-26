# -----------------
# 1. ビルドステージ
# -----------------
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

COPY main.go ./
COPY frontend/ ./frontend/

# スタティックリンクでC言語依存を無くし、ローカルvendorを利用して軽量化
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o proxy-app main.go

# -----------------
# 2. 実行ステージ
# -----------------
FROM scratch

# CA証明書をコピーしてHTTPS通信に対応
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/proxy-app /proxy-app

EXPOSE 3000
CMD ["/proxy-app"]
