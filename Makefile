.PHONY: build run linux test tidy compose-up compose-down clean

build:
	CGO_ENABLED=0 go build -o hub ./cmd/hub

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o hub-linux-amd64 ./cmd/hub

run: build
	./hub -config configs/config.yaml

tidy:
	GOPROXY=https://goproxy.cn,direct go mod tidy

test:
	go vet ./... && go test ./...

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

clean:
	rm -f hub hub.exe hub-linux-amd64
	rm -rf data
