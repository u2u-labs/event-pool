package db

import (
	"fmt"
	"log"

	"event-pool/prisma/db"
	// prisma "github.com/steebchen/prisma-client-go"
)

// NewClient creates a new Prisma client
func NewClient(options ...func(config *db.PrismaConfig)) (*db.PrismaClient, error) {
	// Set up Prisma client
	client := db.NewClient(options...)

	// Connect to the database
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Connected to database")
	return client, nil
}

// Close closes the Prisma client
func Close(client *db.PrismaClient) {
	if err := client.Disconnect(); err != nil {
		log.Printf("Error disconnecting from database: %v", err)
	}
}
