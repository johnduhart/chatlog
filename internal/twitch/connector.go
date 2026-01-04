package twitch

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gempir/go-twitch-irc/v4"
	"github.com/john/chatlog/internal/message"
)

// Message represents a Twitch chat message
type Message struct {
	Timestamp string `json:"timestamp"`
	Channel   string `json:"channel"`
	Username  string `json:"username"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	Badges    string `json:"badges,omitempty"`
}

// Connector manages Twitch chat connections
type Connector struct {
	username string
	oauth    string
	channels []string
	client   *twitch.Client
}

// New creates a new Twitch connector
func New(username, oauth string, channels []string) *Connector {
	return &Connector{
		username: username,
		oauth:    oauth,
		channels: channels,
	}
}

// Start begins listening to Twitch chat
func (c *Connector) Start(ctx context.Context, messageChan chan<- message.Message) error {
	// Create Twitch IRC client
	c.client = twitch.NewClient(c.username, c.oauth)

	// Set up message handler
	c.client.OnPrivateMessage(func(msg twitch.PrivateMessage) {
		// Convert to our Message format
		badges := formatBadges(msg.User.Badges)

		chatMessage := message.Message{
			Platform:  "twitch",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Channel:   strings.TrimPrefix(msg.Channel, "#"),
			Username:  msg.User.DisplayName,
			UserID:    msg.User.ID,
			Message:   msg.Message,
			Badges:    badges,
		}

		// Send to message channel
		select {
		case messageChan <- chatMessage:
		case <-ctx.Done():
			return
		}
	})

	// Set up connection event handlers
	c.client.OnConnect(func() {
		log.Println("Connected to Twitch IRC")
	})

	c.client.OnReconnectMessage(func(msg twitch.ReconnectMessage) {
		log.Println("Reconnecting to Twitch IRC...")
	})

	// Join all channels
	for _, channel := range c.channels {
		c.client.Join(channel)
		log.Printf("Joined channel: %s", channel)
	}

	// Start the client in a goroutine
	go func() {
		if err := c.client.Connect(); err != nil {
			log.Printf("Twitch IRC connection error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Disconnect gracefully
	log.Println("Disconnecting from Twitch IRC...")
	c.client.Disconnect()

	return ctx.Err()
}

// formatBadges converts the badges map to a comma-separated string
func formatBadges(badges map[string]int) string {
	if len(badges) == 0 {
		return ""
	}

	var parts []string
	for badge := range badges {
		parts = append(parts, badge)
	}

	return strings.Join(parts, ",")
}
