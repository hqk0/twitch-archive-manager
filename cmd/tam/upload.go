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
		workspaceDir, err := cfg.GetEffectiveWorkspaceDir()
		if err != nil {
			log.Fatalf("Error determining workspace: %v", err)
		}
		d1 := db.NewD1Client(cfg)
		ctx := context.Background()

		// Path to raw video: workspace/ID/ID.mp4
		videoPath := filepath.Join(workspaceDir, args[0], fmt.Sprintf("%d.mp4", vodID))

		// Raw video upload: Simple title and empty description
		uploadTitle := strconv.FormatInt(vodID, 10)
		description := ""

		fmt.Printf("[%d] Uploading RAW video to YouTube...\n", vodID)
		if cfg.HasD1() {
			d1.UpdateStatusRaw(vodID, 3) // uploading
		}

		yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
		if err != nil {
			log.Fatalf("Failed to initialize YouTube client: %v", err)
		}

		ytID, err := yt.UploadVideo(videoPath, uploadTitle, description, "unlisted")
		if err != nil {
			if cfg.HasD1() {
				d1.UpdateStatusRaw(vodID, 2) // reset to stored
			}
			log.Fatalf("Upload failed: %v", err)
		}

		if cfg.HasD1() {
			d1.UpdateStatusRaw(vodID, 4) // uploaded
			sql := "UPDATE videos SET yt_id = ? WHERE id = ?;"
			d1.Query(sql, []interface{}{ytID, vodID})
		}

		fmt.Printf("[%d] SUCCESS! Raw video uploaded. YouTube ID: %s\n", vodID, ytID)
	},
}

func init() {
	rootCmd.AddCommand(uploadRawCmd)
}
