package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func SendRequestCreateProfile(userID uuid.UUID, userPort, internalSecret string) error {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	payload := map[string]interface{}{"user_id": userID.String()}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://user:%s/api/user/profile", userPort),
		bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", internalSecret)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("users service returned status: %d", resp.StatusCode)
	}

	return nil
}
