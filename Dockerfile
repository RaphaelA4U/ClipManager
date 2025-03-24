# Stage 1: Build the application
FROM golang:1.20-alpine AS builder

# Install git and CA certificates for module downloads
RUN apk add --no-cache git ca-certificates

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy entire source code first so we can initialize modules from scratch
COPY . .

# Initialize a fresh modules setup
RUN rm -f go.sum && \
    go mod tidy && \
    go mod download all

# Build the Go app with dependency resolution
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Stage 2: Final image with FFmpeg installed directly
FROM jrottenberg/ffmpeg:4.4-alpine AS final

# Install ca-certificates
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder and set executable permissions
COPY --from=builder /app/main .
RUN chmod +x ./main

# Copy static files and templates
COPY --from=builder /app/templates/ ./templates/
COPY --from=builder /app/static/ ./static/

# Copy .env file
COPY .env ./.env

# Create directory for temporary clips
RUN mkdir -p /app/clips

# Expose the port
EXPOSE 5000

# Override the default ENTRYPOINT from the ffmpeg image
ENTRYPOINT []

# Command to run the executable
CMD ["./main"]