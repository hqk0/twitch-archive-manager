package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/r2"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
	"github.com/hqk0/twitch-archive-manager/internal/video"
	"github.com/spf13/cobra"
)

var burnCmd = &cobra.Command{
	Use:   "burn [vod_id]",
	Short: "Download chat, convert to ASS, and burn to video",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		cfg := config.Load()
		workspaceDir := cfg.GetEffectiveWorkspaceDir()
		d1 := db.NewD1Client(cfg)
		r2Client, err := r2.NewR2Client(cfg)
		if err != nil {
			log.Fatalf("Failed to initialize R2 client: %v", err)
		}

		vodDir := filepath.Join(workspaceDir, fmt.Sprintf("%d", vodID))
		videoPath := filepath.Join(vodDir, fmt.Sprintf("%d.mp4", vodID))
		if _, err := os.Stat(videoPath); os.IsNotExist(err) {
			log.Fatalf("Raw video not found at %s. Please run download first.", videoPath)
		}

		jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", vodID))
		assPath := filepath.Join(vodDir, fmt.Sprintf("%d.ass", vodID))
		burnedPath := filepath.Join(vodDir, fmt.Sprintf("%d_burned.mp4", vodID))

		// 1. Get JSON
		fmt.Printf("Fetching chat data for VOD %d...\n", vodID)
		err = r2Client.DownloadFile(context.Background(), fmt.Sprintf("%d.json", vodID), jsonPath)
		if err != nil {
			fmt.Printf("R2 download failed: %v. Attempting fallback to Twitch...\n", err)
			
			rawTwitchJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", vodID))
			err = twitch.DownloadChatJSON(vodID, rawTwitchJson)
			if err == nil {
				err = twitch.ConvertTwitchJSONToIntegratedJSON(rawTwitchJson, jsonPath)
				os.Remove(rawTwitchJson)
			}

			if err != nil {
				fmt.Printf("Twitch fallback also failed: %v. Using empty chat data.\n", err)
				os.WriteFile(jsonPath, []byte("[]"), 0644)
			}
		}

		// 2. Convert to ASS
		fmt.Println("Converting chat to ASS...")
		assGen := video.NewASSGenerator()
		err = assGen.GenerateFromJSON(jsonPath, assPath)
		if err != nil {
			log.Fatalf("Failed to convert chat to ASS: %v", err)
		}

		// 3. Burn
		fmt.Println("Burning comments to video...")
		err = video.BurnSubtitles(videoPath, assPath, burnedPath)
		if err != nil {
			log.Fatalf("Failed to burn subtitles: %v", err)
		}

		// 4. Update Status (Optional)
		if cfg.HasD1() {
			d1.UpdateStatusBurned(vodID, 2)
		}
		fmt.Printf("Successfully burned video: %s\n", burnedPath)
	},
}

func init() {
	rootCmd.AddCommand(burnCmd)
}
