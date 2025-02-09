FROM golang:1.23-alpine as builder
WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY . .
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o ports-processor cmd/main.go

FROM alpine:latest
RUN apk add --no-cache libc6-compat sqlite

WORKDIR /app
COPY --from=builder /app/ports-processor .
COPY ports.json .

ENTRYPOINT ["./ports-processor"]
