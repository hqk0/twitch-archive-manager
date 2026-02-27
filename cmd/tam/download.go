package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/video"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download [vod_id]",
	Short: "Manually download a VOD from Twitch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		cfg := config.Load()
		workspaceDir := cfg.GetEffectiveWorkspaceDir()
		
		vodDir := filepath.Join(workspaceDir, fmt.Sprintf("%d", vodID))
		os.MkdirAll(vodDir, 0755)

		fmt.Printf("Downloading VOD %d to %s...\n", vodID, vodDir)
		path, err := video.DownloadVOD(vodID, vodDir)
		if err != nil {
			log.Fatalf("Download failed: %v", err)
		}

		fmt.Printf("Successfully downloaded to: %s\n", path)
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
