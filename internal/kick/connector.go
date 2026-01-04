package kick

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	kickchat "github.com/johanvandegriff/kick-chat-wrapper"
	"github.com/john/chatlog/internal/message"
)

// KickChannelResponse represents the API response from Kick
type KickChannelResponse struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Chatroom struct {
		ID int `json:"id"`
	} `json:"chatroom"`
}

// Connector manages Kick chat connections
type Connector struct {
	channels   []string
	channelIDs map[string]int    // channel slug -> chatroom ID
	idToSlug   map[int]string    // chatroom ID -> channel slug (for reverse lookup)
	client     *kickchat.Client
}

// New creates a new Kick connector
func New(channels []string) *Connector {
	return &Connector{
		channels:   channels,
		channelIDs: make(map[string]int),
		idToSlug:   make(map[int]string),
	}
}

// Start begins listening to Kick chat
func (c *Connector) Start(ctx context.Context, messageChan chan<- message.Message) error {
	// Step 1: Resolve all channel names to chatroom IDs
	log.Println("Resolving Kick channel IDs...")
	for _, channelName := range c.channels {
		chatroomID, slug, err := c.resolveChannelID(channelName)
		if err != nil {
			log.Printf("Warning: Failed to resolve Kick channel '%s': %v (skipping)", channelName, err)
			continue
		}
		c.channelIDs[slug] = chatroomID
		c.idToSlug[chatroomID] = slug
		log.Printf("Resolved Kick channel: %s -> ID %d", slug, chatroomID)
	}

	if len(c.channelIDs) == 0 {
		return fmt.Errorf("no valid Kick channels could be resolved")
	}

	// Step 2: Create WebSocket client
	log.Println("Connecting to Kick chat...")
	client, err := kickchat.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Kick client: %w", err)
	}
	c.client = client
	log.Println("Connected to Kick WebSocket")

	// Step 3: Join all chatrooms
	for slug, chatroomID := range c.channelIDs {
		if err := c.client.JoinChannelByID(chatroomID); err != nil {
			log.Printf("Warning: Failed to join Kick channel '%s' (ID %d): %v", slug, chatroomID, err)
			continue
		}
		log.Printf("Joined Kick channel: %s", slug)
	}

	// Step 4: Start listening for messages
	messages := c.client.ListenForMessages()

	// Process messages until context is cancelled
	go func() {
		for {
			select {
			case msg, ok := <-messages:
				if !ok {
					log.Println("Kick message channel closed")
					return
				}

				// Convert Kick message to generic message format
				chatMessage := c.convertMessage(msg)
				if chatMessage == nil {
					continue // Skip invalid messages
				}

				// Send to message channel
				select {
				case messageChan <- *chatMessage:
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Cleanup
	log.Println("Disconnecting from Kick chat...")
	if c.client != nil {
		c.client.Close()
	}

	return ctx.Err()
}

// resolveChannelID fetches channel information from Kick API
func (c *Connector) resolveChannelID(channelName string) (int, string, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", channelName)

	// Create request with User-Agent to avoid CloudFlare blocking
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var channelInfo KickChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&channelInfo); err != nil {
		return 0, "", fmt.Errorf("JSON decode failed: %w", err)
	}

	return channelInfo.Chatroom.ID, channelInfo.Slug, nil
}

// convertMessage converts a Kick ChatMessage to our generic message.Message
func (c *Connector) convertMessage(msg kickchat.ChatMessage) *message.Message {
	// Look up channel slug from chatroom ID
	slug, ok := c.idToSlug[msg.ChatroomID]
	if !ok {
		log.Printf("Warning: Received message from unknown chatroom ID: %d", msg.ChatroomID)
		return nil
	}

	// Format badges
	badges := c.formatBadges(msg.Sender.Identity.Badges)

	return &message.Message{
		Platform:  "kick",
		Timestamp: msg.CreatedAt.Format(time.RFC3339),
		Channel:   slug,
		Username:  msg.Sender.Username,
		UserID:    strconv.Itoa(msg.Sender.ID),
		Message:   msg.Content,
		Badges:    badges,
	}
}

// formatBadges converts Kick badges to a comma-separated string
func (c *Connector) formatBadges(badges []kickchat.Badge) string {
	if len(badges) == 0 {
		return ""
	}

	var parts []string
	for _, badge := range badges {
		// Format as "type:text" if text is available, otherwise just type
		if badge.Text != "" {
			parts = append(parts, fmt.Sprintf("%s:%s", badge.Type, badge.Text))
		} else {
			parts = append(parts, badge.Type)
		}
	}

	return strings.Join(parts, ",")
}
