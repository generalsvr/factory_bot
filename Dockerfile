# Build stage
FROM golang:1.24 AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO enabled
ENV CGO_ENABLED=1
RUN go build -o factory_bot ./main.go

# Runtime stage - using golang Debian image for compatibility
FROM golang:1.24

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/factory_bot .

# Create data directory for SQLite
RUN mkdir -p /app/data

# Run the binary
CMD ["./factory_bot"] 