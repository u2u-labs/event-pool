#!/bin/sh

# Ensure we're in the correct directory
cd /app

# Ensure the binary is executable
chmod +x /app/event-pool

# Run Prisma migrations
echo "Running Prisma migrations..."
go run github.com/steebchen/prisma-client-go migrate deploy

if [ ! -f ./jwt_secret.key ]; then
  echo "jwt_secret.key not found. Generating a new one..."
  LC_CTYPE=C tr -dc A-Za-z0-9 < /dev/urandom | head -c 10 > ./jwt_secret.key
  echo "\njwt_secret.key created."
fi

# Start the application
echo "Starting the application..."
./event-pool serve
