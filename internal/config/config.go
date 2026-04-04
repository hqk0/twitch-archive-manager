package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	WorkspaceDir string
	// Cloudflare (Optional)
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2BucketName      string
	CFAPIToken        string
	CFAccountID       string
	D1DatabaseID      string
	// Twitch (Required for some features)
	TwitchClientID     string
	TwitchClientSecret string
	TwitchChannelID    string
	// Notifications (Optional)
	NtfyURL   string
	NtfyTopic string
	// YouTube Customization
	YTTitleTemplate    string
	YTDescTemplateFile string
	ViewerBaseURL      string
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		WorkspaceDir:       getEnv("WORKSPACE_DIR", "./workspace"),
		R2AccountID:        getEnv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:      getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey:  getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2BucketName:       getEnv("R2_BUCKET_NAME", ""),
		CFAPIToken:         getEnv("CF_API_TOKEN", ""),
		CFAccountID:        getEnv("CF_ACCOUNT_ID", ""),
		D1DatabaseID:       getEnv("D1_DATABASE_ID", ""),
		TwitchClientID:     getEnv("TWITCH_CLIENT_ID", ""),
		TwitchClientSecret: getEnv("TWITCH_CLIENT_SECRET", ""),
		TwitchChannelID:    getEnv("TWITCH_CHANNEL_ID", ""),
		NtfyURL:            getEnv("NTFY_URL", "https://ntfy.sh"),
		NtfyTopic:          getEnv("NTFY_TOPIC", ""),
		YTTitleTemplate:    getEnv("YT_TITLE_TEMPLATE", "%title%【%date%】"),
		YTDescTemplateFile: getEnv("YT_DESC_TEMPLATE_FILE", "description_template.txt"),
		ViewerBaseURL:      getEnv("VIEWER_BASE_URL", ""),
	}
}

func (c *Config) GetEffectiveWorkspaceDir() (string, error) {
	primaryPath, err := filepath.Abs(c.WorkspaceDir)
	if err != nil {
		return "", fmt.Errorf("invalid workspace directory path: %w", err)
	}

	if _, err := os.Stat(primaryPath); os.IsNotExist(err) {
		err = os.MkdirAll(primaryPath, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create workspace directory '%s': %w", primaryPath, err)
		}
	}

	return primaryPath, nil
}

func (c *Config) HasD1() bool {
	return c.CFAPIToken != "" && c.CFAccountID != "" && c.D1DatabaseID != ""
}

func (c *Config) HasR2() bool {
	return c.R2AccountID != "" && c.R2AccessKeyID != "" && c.R2SecretAccessKey != "" && c.R2BucketName != ""
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
