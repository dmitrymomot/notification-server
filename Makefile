.PHONY: build docker push

build:
	@go clean
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o ./.build/app ./

buildh:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix nocgo -o ./.build/healthchecker ./healthcheck

docker:
	@docker rmi -f dmitrymomot/notification-server:$v
	@docker system prune -f
	@docker build --rm -t dmitrymomot/notification-server:$v .

push:
	@docker tag dmitrymomot/notification-server:$v dmitrymomot/notification-server:latest
	@docker push dmitrymomot/notification-server:$v
	@docker push dmitrymomot/notification-server:latest

up:
	@docker run -d \
		-e JWT_SECRET=SUpYlsf770KYgbrJBaEeIwRR3HpU3uXy \
		-e BASIC_TOKEN=JBaEeIwRR3HpU3uXy \
		-e BASIC_AUTH_USER=admin \
		-e BASIC_AUTH_PASSWORD=admin \
		-e SSE_MAX_AGE=48h \
		-e GC_PERIOD=5m \
		-e APP_PORT=8008 \
		-p 8009:8008 \
		--name=sse \
		dmitrymomot/notification-server:$v

down:
	@docker stop sse
	@docker rm sse
	@docker system prune -f --volumes

restart: down build docker up

logs:
	@docker logs sse