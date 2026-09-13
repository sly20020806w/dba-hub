FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN GOPROXY=https://goproxy.cn,direct go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/hub ./cmd/hub

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /out/hub /app/hub
COPY configs/config.example.yaml /app/configs/config.yaml
RUN mkdir -p /app/data && chown -R app:app /app
USER app
EXPOSE 8091
ENTRYPOINT ["/app/hub", "-config", "/app/configs/config.yaml"]
