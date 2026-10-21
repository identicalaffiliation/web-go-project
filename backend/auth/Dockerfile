FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o auth-service ./cmd/auth/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/auth-service .

ENTRYPOINT ["./auth-service"]
CMD ["-config", "config.yml"]