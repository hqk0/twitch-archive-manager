package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
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
		workspaceDir, err := cfg.GetEffectiveWorkspaceDir()
		if err != nil {
			log.Fatalf("Error determining workspace: %v", err)
		}
		
		vodDir := filepath.Join(workspaceDir, fmt.Sprintf("%d", vodID))
		os.MkdirAll(vodDir, 0755)

		fmt.Printf("[%d] Downloading VOD...\n", vodID)
		path, err := video.DownloadVOD(vodID, vodDir)
		if err != nil {
			log.Fatalf("Download failed: %v", err)
		}

		fmt.Printf("[%d] Successfully downloaded to: %s\n", vodID, path)

		// Update or Create D1 record
		if cfg.HasD1() {
			fmt.Printf("[%d] Updating metadata in D1...\n", vodID)
			d1 := db.NewD1Client(cfg)
			metadata, err := twitch.GetVODMetadata(vodID)
			if err != nil {
				log.Printf("Warning: Failed to fetch VOD metadata: %v. Record might be incomplete.", err)
				// Basic fallback record
				metadata = &twitch.Metadata{
					VODID:        vodID,
					Title:        args[0],
					CreatedAtUTC: time.Now().Format(time.RFC3339),
				}
			}

			// Convert twitch.Metadata to db.Video
			createdAt, _ := time.Parse(time.RFC3339, metadata.CreatedAtUTC)
			v := &db.Video{
				ID:           vodID,
				Title:        metadata.Title,
				Category:     metadata.CategoriesStr,
				Duration:     metadata.DurationHMS,
				CreatedAt:    createdAt,
				StatusRaw:    2, // Downloaded/Stored
				StatusBurned: 0,
			}

			if err := d1.SaveMetadata(v); err != nil {
				log.Printf("Error saving metadata to D1: %v", err)
			} else {
				fmt.Printf("[%d] D1 record updated (Status: stored).\n", vodID)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
