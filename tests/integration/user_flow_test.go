package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestUserLifecycle(t *testing.T) {
	baseURL := os.Getenv("INTEGRATION_BASE_URL")
	if baseURL == "" {
		t.Skip("INTEGRATION_BASE_URL not set; skipping integration test")
	}

	client := &http.Client{Timeout: 5 * time.Second}
	username := fmt.Sprintf("it_user_%d", time.Now().UnixNano())
	password := "Passw0rd!"
	device := "integration"
	mobile := fmt.Sprintf("138%08d", time.Now().UnixNano()%100000000)

	// 1. Register
	registerReq := map[string]string{
		"username": username,
		"password": password,
		"mobile":   mobile,
	}
	if err := postJSON(client, baseURL+"/users/register", registerReq, nil, http.StatusOK); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// 2. Login
	loginReq := map[string]string{
		"username": username,
		"password": password,
	}
	headers := map[string]string{"X-Device": device}
	loginResp, err := postJSONWithResp(client, baseURL+"/users/login", loginReq, headers, http.StatusOK)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// 3. Refresh (rotation)
	refreshReq := map[string]string{"refresh_token": loginResp["refresh_token"]}
	refreshResp, err := postJSONWithResp(client, baseURL+"/users/refresh", refreshReq, headers, http.StatusOK)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	// 4. Logout using new access token
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/users/logout", nil)
	req.Header.Set("Authorization", "Bearer "+refreshResp["access_token"])
	req.Header.Set("X-Device", device)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("logout request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout failed: status=%d", resp.StatusCode)
	}
}

func postJSON(client *http.Client, url string, body interface{}, headers map[string]string, expectedStatus int) error {
	_, err := postJSONWithResp(client, url, body, headers, expectedStatus)
	return err
}

func postJSONWithResp(client *http.Client, url string, body interface{}, headers map[string]string, expectedStatus int) (map[string]string, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
