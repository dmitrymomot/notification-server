.PHONY: build docker

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o ./.build/app ./cmd/main.go

docker:
	docker build --rm -t dmitrymomot/notification-server:latest .
	docker push dmitrymomot/notification-server:latest