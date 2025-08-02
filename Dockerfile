FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o proxy ./cmd/proxy

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/proxy .

RUN adduser -D -u 1000 proxy && \
    chown -R proxy:proxy /app

USER proxy

CMD ["./proxy"]