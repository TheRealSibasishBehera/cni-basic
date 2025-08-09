# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN make build

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates iproute2 iptables

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/bin/cni-basic /opt/cni/bin/

# Create CNI directories
RUN mkdir -p /etc/cni/net.d /var/lib/cni

# Copy example configuration
COPY --from=builder /app/examples/ /etc/cni/net.d/

# Make binary executable
RUN chmod +x /opt/cni/bin/cni-basic

CMD ["/opt/cni/bin/cni-basic"]