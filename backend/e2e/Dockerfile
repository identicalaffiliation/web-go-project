FROM golang:1.27-alpine

WORKDIR /e2e
COPY go.mod *_test.go ./

ENTRYPOINT ["go", "test", "-tags=integration", "-count=1", "-timeout=4m", "-v", "."]
