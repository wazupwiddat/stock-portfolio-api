# Stage 1: Build the binary
FROM golang:1.18 as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app, specifying the main package path and setting GOARCH and GOOS
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server cmd/main.go

# Stage 2: Run the binary
FROM alpine:latest

# Set the Current Working Directory inside the container
WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/server .

# Copy the configuration file
COPY config-prod.yml config.yml

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["./server"]
