FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /admin-api ./cmd/admin-api

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /admin-api /admin-api
EXPOSE 8082
CMD ["/admin-api"]
