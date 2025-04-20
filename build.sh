#!/bin/sh
set -e

# Create bin directory if it doesn't exist
mkdir -p bin

# Build for amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/disspro.amd64 main.go

# Build for arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/disspro.arm64 main.go
