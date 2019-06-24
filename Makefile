.PHONY: build docker push

build:
	@go clean
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o ./.build/app ./

buildh:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o ./.build/healthchecker ./healthcheck

docker:
	@docker rmi -f dmitrymomot/notification-server:latest
	@docker system prune -f
	@docker build --rm -t dmitrymomot/notification-server:latest .

push:
	@docker push dmitrymomot/notification-server:latest

up:
	@docker run -d \
		-e JWT_SECRET=SUpYlsf770KYgbrJBaEeIwRR3HpU3uXy \
		-e BASIC_TOKEN=JBaEeIwRR3HpU3uXy \
		-e BASIC_AUTH_USER=admin \
		-e BASIC_AUTH_PASSWORD=admin \
		-e SSE_MAX_AGE=48h \
		-p 8008:8008 \
		--name=sse \
		dmitrymomot/notification-server

down:
	@docker stop sse
	@docker rm sse
	@docker system prune -f --volumes

restart: down build docker up

logs:
	@docker logs sse