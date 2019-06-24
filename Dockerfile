FROM alpine:latest
RUN apk add --update ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*
WORKDIR /
COPY ./.build/healthchecker /
COPY ./.build/app /
COPY ./static /static
ENV APP_PORT=8008
EXPOSE ${APP_PORT}
CMD ["/app"]