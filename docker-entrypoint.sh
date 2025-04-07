#!/bin/sh

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to be ready..."
while ! nc -z postgres 5432; do
  sleep 0.1
done
echo "PostgreSQL is ready!"

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