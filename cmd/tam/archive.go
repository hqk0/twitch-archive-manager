package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/r2"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
	"github.com/hqk0/twitch-archive-manager/internal/video"
	"github.com/hqk0/twitch-archive-manager/internal/youtube"
	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive [vod_id]",
	Short: "Full process: Download, generate Danmaku, burn, and upload to YouTube (No DB required)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		vodID, _ := strconv.ParseInt(args[0], 10, 64)
		cfg := config.Load()
		workspaceDir := cfg.GetEffectiveWorkspaceDir()
		ctx := context.Background()

		vodDir := filepath.Join(workspaceDir, args[0])
		os.MkdirAll(vodDir, 0755)

		// Filenames
		jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", vodID))
		assPath := filepath.Join(vodDir, fmt.Sprintf("%d.ass", vodID))
		burnedPath := filepath.Join(vodDir, fmt.Sprintf("%d_burned.mp4", vodID))

		// 1. Download VOD
		fmt.Printf("[%d] Downloading VOD...\n", vodID)
		videoPath, err := video.DownloadVOD(vodID, vodDir)
		if err != nil {
			log.Fatalf("Failed to download VOD: %v", err)
		}

		// 2. Get Chat (Try R2, then Twitch)
		success := false
		if cfg.HasR2() {
			fmt.Printf("[%d] Attempting to download chat from R2...\n", vodID)
			r2Client, err := r2.NewR2Client(cfg)
			if err == nil {
				err = r2Client.DownloadFile(ctx, fmt.Sprintf("%d.json", vodID), jsonPath)
				if err == nil {
					success = true
				}
			}
		}

		if !success {
			fmt.Printf("[%d] Fetching chat from Twitch...\n", vodID)
			rawJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", vodID))
			if err := twitch.DownloadChatJSON(vodID, rawJson); err == nil {
				_ = twitch.ConvertTwitchJSONToIntegratedJSON(rawJson, jsonPath)
				os.Remove(rawJson)
				success = true
			}
		}

		// 3. ASS Conversion
		fmt.Printf("[%d] Converting to ASS...\n", vodID)
		assGen := video.NewASSGenerator()
		if success {
			_ = assGen.GenerateFromJSON(jsonPath, assPath)
		} else {
			fmt.Println("Warning: No chat found, generating empty ASS")
			os.WriteFile(jsonPath, []byte("[]"), 0644)
			_ = assGen.GenerateFromJSON(jsonPath, assPath)
		}

		// 4. Burn
		fmt.Printf("[%d] Burning comments (Hardware accelerated)...\n", vodID)
		err = video.BurnSubtitles(videoPath, assPath, burnedPath)
		if err != nil {
			log.Fatalf("Burning failed: %v", err)
		}

		// 5. Upload to YouTube
		fmt.Printf("[%d] Uploading to YouTube (Unlisted)...\n", vodID)
		metadata, _ := twitch.GetVODMetadata(vodID)
		if metadata == nil {
			metadata = &twitch.Metadata{VODID: vodID, Title: args[0]}
		}
		
		yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
		if err != nil {
			fmt.Printf("YouTube auth failed: %v. Video saved at %s\n", err, burnedPath)
			return
		}

		ytID, err := yt.UploadVideo(burnedPath, twitch.GenerateUploadTitle(metadata, cfg), twitch.GenerateDescription(metadata, cfg), "unlisted")
		if err != nil {
			log.Fatalf("Upload failed: %v", err)
		}

		fmt.Printf("\nSUCCESS! VOD %d archived.\nYouTube URL: https://youtu.be/%s\nLocal File: %s\n", vodID, ytID, burnedPath)
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
