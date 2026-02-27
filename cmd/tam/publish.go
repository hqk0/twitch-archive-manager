package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/youtube"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [vod_id]",
	Short: "Publish a video to YouTube (change from unlisted to public)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		cfg := config.Load()
		d1 := db.NewD1Client(cfg)

		// 1. Get video info from D1
		sql := "SELECT id, title, yt_id_burned FROM videos WHERE id = ?;"
		results, err := d1.Query(sql, []interface{}{vodID})
		if err != nil || len(results) == 0 {
			log.Fatalf("Video not found in D1: %v", err)
		}

		ytID := ""
		if val, ok := results[0]["yt_id_burned"].(string); ok {
			ytID = val
		}

		if ytID == "" {
			log.Fatalf("This video has not been uploaded to YouTube yet (yt_id_burned is empty).")
		}

		// 2. Update YouTube privacy status
		title := results[0]["title"].(string)
		fmt.Printf("Publishing video %s (YouTube ID: %s) to Public...\n", title, ytID)
		yt, err := youtube.NewYouTubeClient(context.Background(), "client_secret.json", "youtube_token.json")
		if err != nil {
			log.Fatalf("Failed to initialize YouTube client: %v", err)
		}

		err = yt.SetPrivacyStatus(ytID, "public")
		if err != nil {
			log.Fatalf("Failed to update YouTube privacy status: %v", err)
		}

		fmt.Println("Successfully published to YouTube!")
	},
}

func init() {
	rootCmd.AddCommand(publishCmd)
}
