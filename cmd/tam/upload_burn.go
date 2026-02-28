package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
	"github.com/hqk0/twitch-archive-manager/internal/youtube"
	"github.com/spf13/cobra"
)

var uploadBurnCmd = &cobra.Command{
	Use:   "upload_burn [vod_id]",
	Short: "Upload burned VOD to YouTube",
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

		// Get Metadata
		metadata, err := twitch.GetVODMetadata(vodID)
		if err != nil {
			log.Printf("Warning: Failed to get VOD metadata: %v", err)
			metadata = &twitch.Metadata{VODID: vodID, Title: args[0]}
		}

		uploadTitle := twitch.GenerateUploadTitle(metadata, cfg)
		description := twitch.GenerateDescription(metadata, cfg)

		// Path to burned video: workspace/ID/ID_burned.mp4
		burnedPath := filepath.Join(workspaceDir, args[0], fmt.Sprintf("%d_burned.mp4", vodID))

		fmt.Printf("[%d] Uploading BURNED video to YouTube...\n", vodID)
		if cfg.HasD1() {
			d1.UpdateStatusBurned(vodID, 3) // uploading
		}

		yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
		if err != nil {
			log.Fatalf("Failed to initialize YouTube client: %v", err)
		}

		ytID, err := yt.UploadVideo(burnedPath, uploadTitle, description, "unlisted")
		if err != nil {
			if cfg.HasD1() {
				d1.UpdateStatusBurned(vodID, 2) // reset to burned
			}
			log.Fatalf("Upload failed: %v", err)
		}

		if cfg.HasD1() {
			d1.UpdateYTIDBurned(vodID, ytID, 4) // uploaded
		}
		fmt.Printf("[%d] SUCCESS! Burned video uploaded. YouTube ID: %s\n", vodID, ytID)
	},
}

func init() {
	rootCmd.AddCommand(uploadBurnCmd)
}
