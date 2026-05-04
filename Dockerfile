# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o network-mapper ./cmd/network-mapper/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /build/network-mapper /usr/local/bin/network-mapper

# Default config mount point
RUN mkdir -p /etc/network-mapper
VOLUME ["/etc/network-mapper"]

EXPOSE 8080

ENTRYPOINT ["network-mapper"]
CMD ["serve", "--config", "/etc/network-mapper/config.yaml", "--port", "8080", "--no-open"]
