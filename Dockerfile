# Stage 1: Build the application
FROM golang:1.20-alpine AS builder

# Install git and CA certificates for module downloads
RUN apk add --no-cache git ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Stage 2: Final image with FFmpeg installed directly
FROM jrottenberg/ffmpeg:4.4-alpine AS final

# Install ca-certificates
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/main .

# Copy static files and templates
COPY --from=builder /app/templates/ ./templates/
COPY --from=builder /app/static/ ./static/

# Copy .env file
COPY .env ./.env

# Create directory for temporary clips
RUN mkdir -p /app/clips

# Expose the port
EXPOSE 5000

# Command to run the executable
CMD ["./main"]