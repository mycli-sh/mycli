FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./api/cmd/api

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/api /usr/local/bin/api

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
