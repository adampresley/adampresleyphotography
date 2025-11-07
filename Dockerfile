# Start from golang base image
FROM golang:1.25-alpine3.21 AS builder
EXPOSE 8081

# Set the current working directory inside the container
WORKDIR /build

# Install necessary packages
RUN apk add --no-cache make

# Copy go.mod, go.sum files and download deps
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy sources to the working directory and build
COPY . .
RUN echo "Building app" && make build-linux

# Start a new stage from debian
FROM alpine:3.22.1
LABEL org.opencontainers.image.source=https://github.com/adampresley/adampresleyphotography

WORKDIR /dist

# Copy the build artifacts from the previous stage
COPY --from=builder /build/cmd/website/adampresleyphotography .

# Run the executable
ENTRYPOINT ["./adampresleyphotography"]

