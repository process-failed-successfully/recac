package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	channelID := os.Getenv("DISCORD_CHANNEL_ID")

	fmt.Println("--- Discord Debugger ---")

	if token == "" {
		fmt.Println("❌ DISCORD_BOT_TOKEN is not set")
		return
	} else {
		fmt.Println("✅ DISCORD_BOT_TOKEN is set (length:", len(token), ")")
	}

	if channelID == "" {
		fmt.Println("❌ DISCORD_CHANNEL_ID is not set")
		return
	} else {
		fmt.Println("✅ DISCORD_CHANNEL_ID is set:", channelID)
	}

	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	payload := map[string]string{
		"content": "🔍 Debug message from Recac Debugger at " + time.Now().Format(time.RFC3339),
	}

	// 1. Send Message
	fmt.Println("\nAttempting to send message...")
	msgID, err := sendRequest("POST", url, token, payload)
	if err != nil {
		fmt.Printf("❌ Failed to send message: %v\n", err)
		return
	}
	fmt.Printf("✅ Message sent successfully! ID: %s\n", msgID)

	// 2. Add Reaction (verify granular permissions)
	fmt.Println("\nAttempting to add reaction...")
	reactionURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s/reactions/%%E2%%9C%%85/@me", channelID, msgID)
	_, err = sendRequest("PUT", reactionURL, token, nil)
	if err != nil {
		fmt.Printf("❌ Failed to add reaction: %v\n", err)
		fmt.Println("⚠️  Check if the Bot has 'Add Reactions' and 'Read Message History' permissions.")
		return
	}
	fmt.Println("✅ Reaction added successfully!")
	fmt.Println("\n🎉 Discord integration appears to be working correctly with these credentials.")
}

func sendRequest(method, url, token string, payload interface{}) (string, error) {
	var body io.Reader
	if payload != nil {
		jsonBody, _ := json.Marshal(payload)
		body = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse ID if available (for message send)
	var data struct {
		ID string `json:"id"`
	}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &data)
	}

	return data.ID, nil
}
