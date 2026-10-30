FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o likes-outbox-worker ./cmd/likes-worker/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/likes-outbox-worker .

ENTRYPOINT ["./likes-outbox-worker"]
CMD ["-config", "config.yml"]