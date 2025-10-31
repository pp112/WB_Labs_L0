FROM golang:1.24-alpine AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/

RUN go build -o order-service ./cmd/service

FROM scratch
COPY --from=build /app/order-service /order-service
COPY --from=build /app/web /web

EXPOSE 8080
ENTRYPOINT ["/order-service"]
