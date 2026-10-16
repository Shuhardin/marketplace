FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o graphql-gateway .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/graphql-gateway .
CMD ["./graphql-gateway"]