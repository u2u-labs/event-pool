package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	instance *Server
	once     sync.Once
)

type Server struct {
	config *Config
	client mqtt.Client
	mu     sync.RWMutex
}

type Config struct {
	BrokerURL      string
	ClientID       string
	Username       string
	Password       string
	QoS            byte
	CleanSession   bool
	PingInterval   time.Duration
	ConnectTimeout time.Duration
}

func NewServer(config *Config) (*Server, error) {
	var initErr error
	once.Do(func() {
		opts := mqtt.NewClientOptions().
			AddBroker(config.BrokerURL).
			SetClientID(config.ClientID).
			SetUsername(config.Username).
			SetPassword(config.Password).
			SetCleanSession(config.CleanSession).
			SetPingTimeout(config.PingInterval).
			SetConnectTimeout(config.ConnectTimeout).
			SetAutoReconnect(true).
			SetMaxReconnectInterval(1 * time.Minute).
			SetConnectionLostHandler(func(client mqtt.Client, err error) {
				log.Printf("Connection lost: %v", err)
			}).
			SetOnConnectHandler(func(client mqtt.Client) {
				log.Printf("Connected to MQTT broker")
			})

		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			initErr = fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
			return
		}

		instance = &Server{
			config: config,
			client: client,
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	return instance, nil
}

func (s *Server) Close() {
	if s.client.IsConnected() {
		s.client.Disconnect(250)
	}
}

// RegisterTopic is used to register a topic for tracking purposes
// This doesn't actually subscribe to the topic, just keeps track of it
func (s *Server) RegisterTopic(chainID int, contract, event string) {
	topic := fmt.Sprintf("events/%d/%s/%s", chainID, contract, event)
	log.Printf("Registered topic: %s", topic)
}

// UnregisterTopic is used to unregister a topic for tracking purposes
func (s *Server) UnregisterTopic(chainID int, contract, event string) {
	topic := fmt.Sprintf("events/%d/%s/%s", chainID, contract, event)
	log.Printf("Unregistered topic: %s", topic)
}

// BroadcastEvent publishes an event to the MQTT broker
func (s *Server) BroadcastEvent(chainID int, contract, event string, data interface{}) error {
	topic := fmt.Sprintf("events/%d/%s/%s", chainID, contract, event)

	message, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	token := s.client.Publish(topic, s.config.QoS, false, message)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, token.Error())
	}

	log.Printf("Published event to topic: %s", topic)
	return nil
}
