FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /api .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /api /api
EXPOSE 8080
CMD ["/api"]
