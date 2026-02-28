package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"golang.org/x/sys/unix"
)

type Config struct {
	WorkspaceDir       string
	AltWorkspaceDir    string
	ThresholdGB        int
	// Cloudflare (Optional)
	R2AccountID        string
	R2AccessKeyID      string
	R2SecretAccessKey  string
	R2BucketName       string
	CFAPIToken         string
	CFAccountID        string
	D1DatabaseID       string
	// Twitch (Required for some features)
	TwitchClientID     string
	TwitchClientSecret string
	TwitchChannelID    string
	// Notifications (Optional)
	NtfyURL            string
	NtfyTopic          string
	// YouTube Customization
	YTTitleTemplate    string
	YTDescTemplateFile string
	ViewerBaseURL      string
}

func Load() *Config {
	_ = godotenv.Load()

	threshold := 50
	if val, ok := os.LookupEnv("LOCAL_THRESHOLD_GB"); ok {
		fmt.Sscanf(val, "%d", &threshold)
	}

	return &Config{
		WorkspaceDir:       "./workspace", // Default primary workspace
		AltWorkspaceDir:    getEnv("ALT_WORKSPACE_DIR", ""), // Optional alternative (e.g. SSD)
		ThresholdGB:        threshold,
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
	// Check free space of current directory (.)
	var stat unix.Statfs_t
	wd, _ := os.Getwd()
	err := unix.Statfs(wd, &stat)
	
	// Default to primary workspace
	primaryPath, _ := filepath.Abs(c.WorkspaceDir)

	// If space check fails or no Alt dir is set, use primary workspace
	if err != nil || c.AltWorkspaceDir == "" {
		if _, err := os.Stat(primaryPath); os.IsNotExist(err) {
			return "", fmt.Errorf("workspace directory '%s' not found. Please create it or check your configuration", primaryPath)
		}
		return primaryPath, nil
	}

	freeBytes := stat.Bavail * uint64(stat.Bsize)
	freeGB := int(freeBytes / (1024 * 1024 * 1024))

	// If local space is low, use alternative workspace (SSD)
	if freeGB < c.ThresholdGB {
		if _, err := os.Stat(c.AltWorkspaceDir); os.IsNotExist(err) {
			return "", fmt.Errorf("local disk space is low (%d GB free, threshold: %d GB), but alternative workspace (SSD) '%s' is not connected or not found", freeGB, c.ThresholdGB, c.AltWorkspaceDir)
		}
		return c.AltWorkspaceDir, nil
	}

	if _, err := os.Stat(primaryPath); os.IsNotExist(err) {
		// If primary doesn't exist, try to create it or fall back to Alt if available
		os.MkdirAll(primaryPath, 0755)
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
