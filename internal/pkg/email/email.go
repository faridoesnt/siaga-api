package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type ShiftSwapNotificationData struct {
	RequesterName string
	TargetName    string
	ShiftDate     string
	OriginalShift string
	NewShift      string
	Timestamp     string
}

func SendShiftSwapNotification(ctx context.Context, data ShiftSwapNotificationData) {
	apiKey := os.Getenv("RESEND_API_KEY")
	toList := os.Getenv("ADMIN_NOTIFICATION_EMAIL")
	if apiKey == "" || toList == "" {
		return
	}

	subject := "[SIAGA] Shift Swap Completed"

	body := fmt.Sprintf(
		"Shift swap completed:\n\nRequester: %s\nTarget: %s\nDate: %s\nOriginal shift: %s\nNew shift: %s\nTimestamp: %s\n",
		data.RequesterName,
		data.TargetName,
		data.ShiftDate,
		data.OriginalShift,
		data.NewShift,
		data.Timestamp,
	)

	var recipients []string
	for _, addr := range strings.Split(toList, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			recipients = append(recipients, addr)
		}
	}
	if len(recipients) == 0 {
		return
	}

	payload := map[string]interface{}{
		"from":    "SIAGA System <no-reply@siaga.local>",
		"to":      recipients,
		"subject": subject,
		"text":    body,
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal resend payload: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(buf))
	if err != nil {
		log.Printf("failed to build resend request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("failed to send resend email: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("resend email returned non-2xx status: %s", resp.Status)
	}
}
