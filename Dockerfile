FROM golang:1.23-alpine AS builder
RUN apk update && apk add build-base cmake gcc git
WORKDIR /app

# Copy only the files needed for dependency resolution first
COPY go.mod go.sum ./
COPY prisma ./prisma/
COPY pkg ./pkg/
COPY internal ./internal/

# Initialize Go module and install dependencies
RUN go mod download
RUN go get github.com/steebchen/prisma-client-go
RUN go run github.com/steebchen/prisma-client-go generate

# Now copy the rest of the application
COPY . .

# Update dependencies
RUN go mod tidy

# Build the application
RUN go build -ldflags -w -o event-pool

FROM golang:1.23-alpine
RUN apk add ca-certificates curl
WORKDIR /app

# Copy the entire application from builder
COPY --from=builder /app /app

# Install Prisma CLI in the final image
RUN go get github.com/steebchen/prisma-client-go

# Set the entrypoint to run migrations and then start the application
COPY docker-entrypoint.sh /app/
RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]

