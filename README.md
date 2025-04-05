# Event Pool

Event Pool is a service that listens to specific smart contract events on supported L1 chains, stores them, and pushes them to subscribing clients in real-time.

## Features

- Chain Registration API for contract event monitoring
- Real-time event listening and publishing
- WebSocket-based client subscriptions
- Historical event backfilling
- Support for multiple chains
- Scalable architecture with Redis and PostgreSQL

## Tech Stack

- Go 1.21+
- Cobra for CLI
- Viper for configuration
- Prisma for database access
- go-ethereum for blockchain interaction
- asynq for background jobs
- WebSocket for client connections
- Redis for pub/sub and job queue
- PostgreSQL for data storage

## Prerequisites

- Go 1.21 or later
- PostgreSQL 12 or later
- Redis 6 or later
- Access to Ethereum node RPC endpoints

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/event-pool.git
cd event-pool
```

2. Install dependencies:
```bash
go mod download
```

3. Set up the database:
```bash
# Create PostgreSQL database
createdb event_pool

# Run Prisma migrations
prisma migrate dev
```

4. Configure the application:
```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your settings
```

## Configuration

The application is configured using `config.yaml`. Key configuration sections:

- `server`: HTTP server settings
- `database`: PostgreSQL connection details
- `redis`: Redis connection settings
- `ethereum`: Chain configurations
- `websocket`: WebSocket server settings
- `asynq`: Background job settings

## Usage

### Starting the Service

```bash
go run cmd/event-pool/main.go serve
```

### Registering a Contract

```bash
curl -X POST http://localhost:8080/api/v1/contracts \
  -H "Content-Type: application/json" \
  -d '{
    "chainId": 1,
    "contractAddress": "0x...",
    "eventSignature": "Transfer(address,address,uint256)",
    "startBlock": 12345678
  }'
```

### Subscribing to Events

Connect to the WebSocket endpoint:
```
wss://your-domain/1/0x.../Transfer
```

## Development

### Project Structure

```
.
├── cmd/
│   └── event-pool/        # CLI application
├── internal/
│   ├── config/           # Configuration
│   ├── listener/         # Event listener
│   ├── models/           # Data models
│   ├── pubsub/           # Pub/sub system
│   └── worker/           # Background jobs
├── pkg/
│   ├── ethereum/         # Ethereum client
│   └── websocket/        # WebSocket server
└── prisma/               # Database schema
```

### Running Tests

```bash
go test ./...
```

## License

MIT License 