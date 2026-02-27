package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/youtube"
	"github.com/spf13/cobra"
)

var uploadRawCmd = &cobra.Command{
	Use:   "upload [vod_id]",
	Short: "Upload raw VOD to YouTube",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		cfg := config.Load()
		workspaceDir := cfg.GetEffectiveWorkspaceDir()
		d1 := db.NewD1Client(cfg)
		ctx := context.Background()

		// Raw video upload: Simple title and empty description
		uploadTitle := strconv.FormatInt(vodID, 10)
		description := ""

		// Path to raw video
		videoPath := filepath.Join(workspaceDir, strconv.FormatInt(vodID, 10), fmt.Sprintf("%d.mp4", vodID))

		fmt.Printf("Uploading RAW video %d to YouTube...\n", vodID)
		d1.UpdateStatusRaw(vodID, 3) // uploading

		yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
		if err != nil {
			log.Fatalf("Failed to initialize YouTube client: %v", err)
		}

		ytID, err := yt.UploadVideo(videoPath, uploadTitle, description, "unlisted")
		if err != nil {
			d1.UpdateStatusRaw(vodID, 2) // reset to stored
			log.Fatalf("Upload failed: %v", err)
		}

		d1.UpdateStatusRaw(vodID, 4) // uploaded
		// In existing schema, yt_id is for raw
		sql := "UPDATE videos SET yt_id = ? WHERE id = ?;"
		d1.Query(sql, []interface{}{ytID, vodID})

		fmt.Printf("Successfully uploaded raw video! YouTube ID: %s\n", ytID)
	},
}

func init() {
	rootCmd.AddCommand(uploadRawCmd)
}
