# Use official Golang image as the base image
FROM golang:1.23-alpine

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files (if they exist)
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o main .

# Expose port (adjust if your app uses a different port)
EXPOSE 8080

# Run the application
CMD ["./main"]
