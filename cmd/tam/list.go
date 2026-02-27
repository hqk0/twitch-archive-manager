package main

import (
	"fmt"
	"log"
	"time"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List VOD tasks from D1",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		if !cfg.HasD1() {
			log.Fatalf("Database (D1) is not configured. Please set the required environment variables.")
		}
		d1 := db.NewD1Client(cfg)

		tasks, err := d1.GetAllVideos()
		if err != nil {
			log.Fatalf("Failed to get tasks: %v", err)
		}

		fmt.Printf("%-12s | %-10s | %-13s | %-30s | %-20s\n", "VOD ID", "Raw Stat", "Burned Stat", "Title", "Created At")
		fmt.Println("-----------------------------------------------------------------------------------------------------------------")
		for _, t := range tasks {
			fmt.Printf("%-12d | %-10d | %-13d | %-30.30s | %-20s\n", t.ID, t.StatusRaw, t.StatusBurned, t.Title, t.CreatedAt.Format(time.RFC3339))
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
