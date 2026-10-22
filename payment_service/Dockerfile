FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o payment-service .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/payment-service .
CMD ["./payment-service"]