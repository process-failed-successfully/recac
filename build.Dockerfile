FROM golang:1.24-alpine

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache make git

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Default command
CMD ["go", "build", "-o", "recac", "./cmd/recac"]
