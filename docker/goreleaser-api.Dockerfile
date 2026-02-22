FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

COPY api /usr/local/bin/api

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
