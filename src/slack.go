package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// SlackMessage represents the structure of a message to be sent to Slack
type SlackMessage struct {
	Text string `json:"text"`
}

// SendSlackMessage sends a text message to a Slack channel using a webhook URL
func SendSlackMessage(message string) error {
	webhookURL := os.Getenv("SLACK_WEBHOOK")
	if webhookURL == "" {
		return fmt.Errorf("SLACK_WEBHOOK environment variable is not set")
	}

	slackMessage := SlackMessage{
		Text: message,
	}

	jsonData, err := json.Marshal(slackMessage)
	if err != nil {
		return fmt.Errorf("error marshaling Slack message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending message to Slack: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned non-200 status code: %d", resp.StatusCode)
	}

	return nil
}
