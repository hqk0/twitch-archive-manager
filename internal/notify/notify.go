package notify

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hqk0/twitch-archive-manager/internal/config"
)

func SendNotification(cfg *config.Config, title, message string, priority string) error {
	if cfg.NtfyTopic == "" {
		return nil
	}

	url := fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.NtfyURL, "/"), cfg.NtfyTopic)
	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return err
	}

	req.Header.Set("Title", title)
	if priority != "" {
		req.Header.Set("Priority", priority)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ntfy error: %s", resp.Status)
	}

	return nil
}
