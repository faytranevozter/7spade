FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /ws .

FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /ws /ws
EXPOSE 8081
CMD ["/ws"]
