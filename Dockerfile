FROM golang:1.23-alpine AS builder
WORKDIR /app
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o transbridge

FROM alpine:latest
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/transbridge .
COPY --from=builder /app/config.example.yml ./config.yml
EXPOSE 8080
CMD ["./transbridge", "-config", "config.yml"]
