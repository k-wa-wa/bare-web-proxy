FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY benchmark/mockserver/main.go ./benchmark/mockserver/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o mock-server benchmark/mockserver/main.go

FROM alpine:latest
COPY --from=builder /app/mock-server /mock-server
EXPOSE 3003
ENTRYPOINT ["/mock-server"]
