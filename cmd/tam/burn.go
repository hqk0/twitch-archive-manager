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
		workspaceDir, err := cfg.GetEffectiveWorkspaceDir()
		if err != nil {
			log.Fatalf("Error determining workspace: %v", err)
		}
		d1 := db.NewD1Client(cfg)
		r2Client, err := r2.NewR2Client(cfg)
		
		vodDir := filepath.Join(workspaceDir, args[0])
		videoPath := filepath.Join(vodDir, fmt.Sprintf("%d.mp4", vodID))
		if _, err := os.Stat(videoPath); os.IsNotExist(err) {
			log.Fatalf("Raw video not found at %s. Please run download first.", videoPath)
		}

		jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", vodID))
		assPath := filepath.Join(vodDir, fmt.Sprintf("%d.ass", vodID))
		burnedPath := filepath.Join(vodDir, fmt.Sprintf("%d_burned.mp4", vodID))

		// 1. Get JSON (R2 -> Twitch fallback)
		fmt.Printf("[%d] Fetching chat data...\n", vodID)
		success := false
		if cfg.HasR2() {
			err = r2Client.DownloadFile(context.Background(), fmt.Sprintf("%d.json", vodID), jsonPath)
			if err == nil {
				success = true
			}
		}

		if !success {
			fmt.Printf("[%d] Fetching chat from Twitch (R2 failed or not configured)...\n", vodID)
			rawTwitchJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", vodID))
			if err := twitch.DownloadChatJSON(vodID, rawTwitchJson); err == nil {
				_ = twitch.ConvertTwitchJSONToIntegratedJSON(rawTwitchJson, jsonPath)
				os.Remove(rawTwitchJson)
				success = true
			}
		}

		if !success {
			fmt.Printf("[%d] Warning: No chat data could be fetched. Using empty data.\n", vodID)
			os.WriteFile(jsonPath, []byte("[]"), 0644)
		}

		// 2. Convert to ASS
		fmt.Printf("[%d] Converting chat to ASS...\n", vodID)
		assGen := video.NewASSGenerator()
		err = assGen.GenerateFromJSON(jsonPath, assPath)
		if err != nil {
			log.Fatalf("Failed to convert chat to ASS: %v", err)
		}

		// 3. Burn
		fmt.Printf("[%d] Burning comments to video (Hardware accelerated)...\n", vodID)
		err = video.BurnSubtitles(videoPath, assPath, burnedPath)
		if err != nil {
			log.Fatalf("Failed to burn subtitles: %v", err)
		}

		// 4. Update Status (Optional)
		if cfg.HasD1() {
			d1.UpdateStatusBurned(vodID, 2)
		}
		fmt.Printf("[%d] SUCCESS! Burned video saved at: %s\n", vodID, burnedPath)
	},
}

func init() {
	rootCmd.AddCommand(burnCmd)
}
