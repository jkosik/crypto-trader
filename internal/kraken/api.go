package kraken

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GetKrakenSignature generates the API signature for private Kraken API endpoints
func GetKrakenSignature(urlPath string, payload string, secret string) (string, error) {
	// Parse the JSON payload
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &jsonData); err != nil {
		return "", fmt.Errorf("Failed to parse JSON payload: %v", err)
	}

	// Get nonce from the parsed JSON
	nonce, ok := jsonData["nonce"].(string)
	if !ok {
		return "", fmt.Errorf("Nonce not found in payload or not a string")
	}

	// Create the encoded data string
	encodedData := nonce + payload

	sha := sha256.New()
	sha.Write([]byte(encodedData))
	shaSum := sha.Sum(nil)

	message := append([]byte(urlPath), shaSum...)

	decodedSecret, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("Failed to decode secret: %v", err)
	}

	mac := hmac.New(sha512.New, decodedSecret)
	mac.Write(message)
	macSum := mac.Sum(nil)
	sigDigest := base64.StdEncoding.EncodeToString(macSum)
	return sigDigest, nil
}

// MakePublicRequest makes a request to Kraken's public API endpoints
func MakePublicRequest(url string, method string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

// MakePrivateRequest makes a request to Kraken's private API endpoints with auth
func MakePrivateRequest(url string, method string, payload string, apiKey string, signature string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add headers for private API
	req.Header.Add("API-Key", apiKey)
	req.Header.Add("API-Sign", signature)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}
