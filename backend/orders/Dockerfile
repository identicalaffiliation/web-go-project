FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o orders-service ./cmd/orders/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/orders-service .

ENTRYPOINT ["./orders-service"]
CMD ["-config", "config.yml"]