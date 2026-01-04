package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// KickChannelResponse represents the API response from Kick
type KickChannelResponse struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Chatroom struct {
		ID int `json:"id"`
	} `json:"chatroom"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: resolve-kick-channels <channel1> [channel2] ...")
		fmt.Println("\nExample:")
		fmt.Println("  resolve-kick-channels paymoneywubby xqc")
		os.Exit(1)
	}

	channels := os.Args[1:]
	fmt.Printf("Resolving %d Kick channel(s)...\n\n", len(channels))

	results := make(map[string]int)
	errors := make(map[string]string)

	for _, channel := range channels {
		chatroomID, err := resolveChannel(channel)
		if err != nil {
			errors[channel] = err.Error()
		} else {
			results[channel] = chatroomID
		}
	}

	// Print results
	if len(results) > 0 {
		fmt.Println("✓ Successfully resolved:")
		fmt.Println("---")
		for slug, id := range results {
			fmt.Printf("%s: %d\n", slug, id)
		}
		fmt.Println()
	}

	if len(errors) > 0 {
		fmt.Println("✗ Failed to resolve:")
		fmt.Println("---")
		for slug, err := range errors {
			fmt.Printf("%s: %s\n", slug, err)
		}
		fmt.Println()
	}

	// Print YAML config snippet
	if len(results) > 0 {
		fmt.Println("Add this to your config.yaml:")
		fmt.Println("---")
		fmt.Println("kick:")
		fmt.Println("  enabled: true")
		fmt.Println("  channels:")
		for slug, id := range results {
			fmt.Printf("    - slug: %s\n", slug)
			fmt.Printf("      chatroom_id: %d\n", id)
		}
	}
}

func resolveChannel(channelName string) (int, error) {
	url := fmt.Sprintf("https://kick.com/api/v2/channels/%s", channelName)

	// Create request with headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set comprehensive browser headers (excluding Accept-Encoding to let Go handle it automatically)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://kick.com/")
	req.Header.Set("Origin", "https://kick.com")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("sec-ch-ua", `"Chromium";v="143", "Not.A/Brand";v="24", "Google Chrome";v="143"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Go's http client automatically decompresses gzip responses
	var channelInfo KickChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&channelInfo); err != nil {
		// Debug: try reading raw body
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("JSON decode failed: %w (first bytes: %q)", err, string(body[:min(100, len(body))]))
	}

	return channelInfo.Chatroom.ID, nil
}
