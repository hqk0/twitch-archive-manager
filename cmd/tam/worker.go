package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/db"
	"github.com/hqk0/twitch-archive-manager/internal/notify"
	"github.com/hqk0/twitch-archive-manager/internal/r2"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
	"github.com/hqk0/twitch-archive-manager/internal/video"
	"github.com/hqk0/twitch-archive-manager/internal/youtube"
	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Run the archival worker",
	Run: func(cmd *cobra.Command, args []string) {
		runWorker()
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)
}

func runWorker() {
	cfg := config.Load()
	workspaceDir := cfg.GetEffectiveWorkspaceDir()

	// Check if D1 is configured
	if !cfg.HasD1() {
		log.Fatalf("Worker mode requires Cloudflare D1 configuration. Please set CF_API_TOKEN, CF_ACCOUNT_ID, and D1_DATABASE_ID in your .env file.\nFor a single VOD without a database, use 'tam archive <vod_id>' instead.")
	}

	// 1. Workspace directory check
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		log.Printf("Workspace directory %s not found. Skipping...", workspaceDir)
		return
	}

	d1 := db.NewD1Client(cfg)
	r2Client, err := r2.NewR2Client(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize R2 client: %v", err)
	}

	tasks, err := d1.GetPendingTasks()
	if err != nil {
		log.Fatalf("Failed to get tasks from D1: %v", err)
	}

	ctx := context.Background()

	for _, task := range tasks {
		if task.StatusBurned < 2 {
			processPendingTask(ctx, task, cfg, workspaceDir, d1, r2Client)
		} else if task.StatusBurned == 2 {
			processBurnedTask(ctx, task, cfg, workspaceDir, d1)
		}
	}
}

func processPendingTask(ctx context.Context, task db.Video, cfg *config.Config, workspaceDir string, d1 *db.D1Client, r2Client *r2.R2Client) {
	log.Printf("[%d] Processing pending task: %s", task.ID, task.Title)
	notify.SendNotification(cfg, "アーカイブ処理開始", fmt.Sprintf("VOD %d のダウンロードと焼き込みを開始します: %s", task.ID, task.Title), "low")

	// Update status to burning (1)
	d1.UpdateStatusBurned(task.ID, 1)

	// Create directories
	vodDir := filepath.Join(workspaceDir, fmt.Sprintf("%d", task.ID))
	os.MkdirAll(vodDir, 0755)

	// Filenames
	jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", task.ID))
	assPath := filepath.Join(vodDir, fmt.Sprintf("%d.ass", task.ID))
	burnedPath := filepath.Join(vodDir, fmt.Sprintf("%d_burned.mp4", task.ID))

	// 1. Download VOD
	videoPath, err := video.DownloadVOD(task.ID, vodDir)
	if err != nil {
		log.Printf("[%d] Failed to download VOD: %v", task.ID, err)
		return
	}

	// 2. Download JSON from R2
	err = r2Client.DownloadFile(ctx, fmt.Sprintf("%d.json", task.ID), jsonPath)
	if err != nil {
		log.Printf("[%d] R2 download failed: %v. Attempting Twitch fallback...", task.ID, err)
		
		rawTwitchJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", task.ID))
		err = twitch.DownloadChatJSON(task.ID, rawTwitchJson)
		if err == nil {
			err = twitch.ConvertTwitchJSONToIntegratedJSON(rawTwitchJson, jsonPath)
			os.Remove(rawTwitchJson)
		}

		if err != nil {
			log.Printf("[%d] Twitch fallback also failed: %v. Proceeding with empty chat.", task.ID, err)
			os.WriteFile(jsonPath, []byte("[]"), 0644)
		}
	}

	// 3. Convert JSON to ASS
	log.Printf("[%d] Converting chat to ASS...", task.ID)
	assGen := video.NewASSGenerator()
	err = assGen.GenerateFromJSON(jsonPath, assPath)
	if err != nil {
		log.Printf("[%d] Failed to convert chat to ASS: %v", task.ID, err)
		return
	}

	// 4. Burn Comments
	err = video.BurnSubtitles(videoPath, assPath, burnedPath)
	if err != nil {
		log.Printf("[%d] Failed to burn subtitles: %v", task.ID, err)
		return
	}

	// 5. Update Status to burned (2)
	err = d1.UpdateStatusBurned(task.ID, 2)
	if err != nil {
		log.Printf("[%d] Failed to update status to burned: %v", task.ID, err)
	} else {
		notify.SendNotification(cfg, "焼き込み完了", fmt.Sprintf("VOD %d の焼き込みが完了しました。3日後にアップロードされます。", task.ID), "default")
	}
}

func processBurnedTask(ctx context.Context, task db.Video, cfg *config.Config, workspaceDir string, d1 *db.D1Client) {
	// Check if 3 days have passed since END of stream
	duration, _ := twitch.ParseTwitchDuration(task.Duration)
	endTime := task.CreatedAt.Add(duration)

	if time.Since(endTime) < 72*time.Hour {
		return
	}

	log.Printf("[%d] 3 days passed since stream end. Uploading to YouTube...", task.ID)
	
	// Update status to uploading (3)
	d1.UpdateStatusBurned(task.ID, 3)

	// Initialize YouTube client
	yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
	if err != nil {
		log.Printf("[%d] Failed to initialize YouTube client: %v", task.ID, err)
		return
	}

	// 1. Get Metadata for title/description
	metadata, err := twitch.GetVODMetadata(task.ID)
	if err != nil {
		log.Printf("[%d] Failed to get VOD metadata from Twitch: %v", task.ID, err)
		metadata = &twitch.Metadata{
			Title: task.Title,
			VODID: task.ID,
		}
	}

	uploadTitle := twitch.GenerateUploadTitle(metadata, cfg)
	description := twitch.GenerateDescription(metadata, cfg)

	burnedPath := filepath.Join(workspaceDir, fmt.Sprintf("%d", task.ID), fmt.Sprintf("%d_burned.mp4", task.ID))
	ytID, err := yt.UploadVideo(burnedPath, uploadTitle, description, "unlisted")
	if err != nil {
		log.Printf("[%d] Failed to upload to YouTube: %v", task.ID, err)
		d1.UpdateStatusBurned(task.ID, 2)
		return
	}

	// Update Status to uploaded (4) and set yt_id_burned
	err = d1.UpdateYTIDBurned(task.ID, ytID, 4)
	if err != nil {
		log.Printf("[%d] Failed to update status to uploaded: %v", task.ID, err)
	} else {
		notify.SendNotification(cfg, "YouTubeアップロード完了", fmt.Sprintf("VOD %d がYouTubeにアップロードされました。\nURL: https://youtu.be/%s", task.ID, ytID), "high")
	}
}
