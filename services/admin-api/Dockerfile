FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /admin-api ./cmd/admin-api
RUN go build -o /bootstrap-admin ./cmd/bootstrap-admin

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /admin-api /admin-api
COPY --from=builder /bootstrap-admin /bootstrap-admin
EXPOSE 8082
CMD ["/admin-api"]
