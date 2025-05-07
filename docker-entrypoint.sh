#!/bin/sh

# Ensure we're in the correct directory
cd /app

# Ensure the binary is executable
chmod +x /app/event-pool

# Run Prisma migrations
echo "Running Prisma migrations..."
go run github.com/steebchen/prisma-client-go migrate deploy

# Start the application
echo "Starting the application..."
./event-pool serve
