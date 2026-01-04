package message

// Message represents a chat message from any platform (Twitch, Kick, etc.)
type Message struct {
	Platform  string `json:"platform"`           // Platform name: "twitch", "kick", etc.
	Timestamp string `json:"timestamp"`          // Message timestamp in RFC3339 format (UTC)
	Channel   string `json:"channel"`            // Channel name or slug
	Username  string `json:"username"`           // User's display name
	UserID    string `json:"user_id"`            // Platform-specific user ID
	Message   string `json:"message"`            // Chat message content
	Badges    string `json:"badges,omitempty"`   // Comma-separated list of badges
}
