# -----------------
# 1. フロントエンドビルドステージ
# -----------------
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/*.ts frontend/*.css frontend/*.html frontend/*.mjs frontend/tsconfig.json ./
RUN npm run build

# -----------------
# 2. Goビルドステージ
# -----------------
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

COPY cmd/ ./cmd/
COPY internal/ ./internal/

# frontend-builderで生成したdist/でプレースホルダーを上書き
COPY --from=frontend-builder /frontend/dist ./internal/proxy/frontend/dist/

# スタティックリンクでC言語依存を無くし、ローカルvendorを利用して軽量化
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o proxy-app cmd/proxy/main.go

# -----------------
# 3. 実行ステージ
# -----------------
FROM scratch

# CA証明書をコピーしてHTTPS通信に対応
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/proxy-app /proxy-app

EXPOSE 3000
CMD ["/proxy-app"]
