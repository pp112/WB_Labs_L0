FROM golang:1.24-alpine

WORKDIR /app

COPY publish.go .

RUN go mod init publisher && \
    go get github.com/nats-io/stan.go && \
    go mod tidy

ENTRYPOINT ["go", "run", "publish.go"]
