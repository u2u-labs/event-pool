FROM golang:1.23-alpine AS builder
RUN apk update && apk add build-base cmake gcc git
WORKDIR /app
COPY . .

# Initialize Go module if not exists
RUN if [ ! -f go.mod ]; then go mod init event-pool; fi

# Install Go dependencies
RUN go mod tidy
RUN go get github.com/steebchen/prisma-client-go

# Build the application
RUN go build -ldflags -w -o event-pool

FROM golang:1.23-alpine
RUN apk add ca-certificates curl
WORKDIR /app

# Copy the entire application from builder
COPY --from=builder /app /app

# Set the entrypoint to run migrations and then start the application
COPY docker-entrypoint.sh /app/
RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]

