package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
	workspaceDir, err := cfg.GetEffectiveWorkspaceDir()
	if err != nil {
		log.Fatalf("Error determining workspace: %v", err)
	}

	if !cfg.HasD1() {
		log.Fatalf("Worker mode requires Cloudflare D1 configuration. Please set CF_API_TOKEN, CF_ACCOUNT_ID, and D1_DATABASE_ID in your .env file.")
	}

	fmt.Printf("Worker started. Workspace: %s\n", workspaceDir)

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		fmt.Printf("Workspace directory %s not found. SSD might not be mounted. Exiting...\n", workspaceDir)
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

	if len(tasks) == 0 {
		fmt.Println("No pending tasks found in database. Working finished.")
		return
	}

	fmt.Printf("Found %d tasks to check.\n", len(tasks))

	ctx := context.Background()

	for _, task := range tasks {
		if task.StatusBurned < 2 {
			processPendingTask(ctx, task, cfg, workspaceDir, d1, r2Client)
		} else if task.StatusBurned == 2 {
			processBurnedTask(ctx, task, cfg, workspaceDir, d1)
		} else if task.StatusBurned == 3 {
			fmt.Printf("[%d] Task is currently marked as 'uploading' (3). Skipping to avoid conflict.\n", task.ID)
		}
	}

	fmt.Println("Worker cycle completed.")
}

func processPendingTask(ctx context.Context, task db.Video, cfg *config.Config, workspaceDir string, d1 *db.D1Client, r2Client *r2.R2Client) {
	log.Printf("[%d] Processing pending task: %s", task.ID, task.Title)
	notify.SendNotification(cfg, "アーカイブ処理開始", fmt.Sprintf("VOD %d のダウンロードと焼き込みを開始します: %s", task.ID, task.Title), "low")

	d1.UpdateStatusBurned(task.ID, 1)

	vodDir := filepath.Join(workspaceDir, fmt.Sprintf("%d", task.ID))
	os.MkdirAll(vodDir, 0755)

	jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", task.ID))
	assPath := filepath.Join(vodDir, fmt.Sprintf("%d.ass", task.ID))
	burnedPath := filepath.Join(vodDir, fmt.Sprintf("%d_burned.mp4", task.ID))

	// 1. Download VOD
	videoPath, err := video.DownloadVOD(task.ID, vodDir)
	if err != nil {
		log.Printf("[%d] Failed to download VOD: %v", task.ID, err)
		return
	}

	err = d1.UpdateStatusRaw(task.ID, 2)
	if err != nil {
		log.Printf("[%d] Failed to update status to raw: %v", task.ID, err)
	} else {
		notify.SendNotification(cfg, "ダウンロード完了", fmt.Sprintf("VOD %d のダウンロードが完了しました。4日後にアップロードされます。", task.ID), "default")
	}

	// 2. Download JSON from R2 with Twitch fallback
	err = r2Client.DownloadFile(ctx, fmt.Sprintf("%d.json", task.ID), jsonPath)
	if err != nil {
		log.Printf("[%d] R2 download failed: %v. Attempting Twitch fallback...", task.ID, err)
		rawTwitchJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", task.ID))
		if err := twitch.DownloadChatJSON(task.ID, rawTwitchJson); err == nil {
			_ = twitch.ConvertTwitchJSONToIntegratedJSON(rawTwitchJson, jsonPath)
			os.Remove(rawTwitchJson)
		} else {
			log.Printf("[%d] Twitch fallback failed. Using empty chat.", task.ID)
			os.WriteFile(jsonPath, []byte("[]"), 0644)
		}
	}

	// 3. Convert JSON to ASS
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
		notify.SendNotification(cfg, "焼き込み完了", fmt.Sprintf("VOD %d の焼き込みが完了しました。4日後にアップロードされます。", task.ID), "default")
	}
}

func processBurnedTask(ctx context.Context, task db.Video, cfg *config.Config, workspaceDir string, d1 *db.D1Client) {
	duration, _ := twitch.ParseTwitchDuration(task.Duration)
	endTime := task.CreatedAt.Add(duration)
	timeLeft := 96*time.Hour - time.Since(endTime)

	if timeLeft > 0 {
		fmt.Printf("[%d] Waiting for embargo: %.1f hours remaining.\n", task.ID, timeLeft.Hours())
		return
	}

	log.Printf("[%d] 4 days passed since stream end. Uploading to YouTube...", task.ID)
	d1.UpdateStatusBurned(task.ID, 3)

	yt, err := youtube.NewYouTubeClient(ctx, "client_secret.json", "youtube_token.json")
	if err != nil {
		log.Printf("[%d] Failed to initialize YouTube client: %v", task.ID, err)
		d1.UpdateStatusBurned(task.ID, 2)
		return
	}

	metadata, err := twitch.GetVODMetadata(task.ID)
	if err != nil {
		metadata = &twitch.Metadata{Title: task.Title, VODID: task.ID}
	}

	uploadTitle := twitch.GenerateUploadTitle(metadata, cfg)
	description := twitch.GenerateDescription(metadata, cfg)

	// Upload Raw
	rawPath := filepath.Join(workspaceDir, fmt.Sprintf("%d", task.ID), fmt.Sprintf("%d.mp4", task.ID))
	rawTitle := strconv.FormatInt(task.ID, 10)

	d1.UpdateStatusRaw(task.ID, 3)
	rawYtID, err := yt.UploadVideo(rawPath, rawTitle, "", "unlisted")
	if err != nil {
		log.Printf("[%d] Failed to upload raw to YouTube: %v", task.ID, err)
		d1.UpdateStatusRaw(task.ID, 2)
	} else {
		d1.UpdateStatusRaw(task.ID, 4)
		sql := "UPDATE videos SET yt_id = ? WHERE id = ?;"
		d1.Query(sql, []interface{}{rawYtID, task.ID})
		log.Printf("[%d] Raw video uploaded. YouTube ID: %s", task.ID, rawYtID)
	}

	// Upload Burned
	burnedPath := filepath.Join(workspaceDir, fmt.Sprintf("%d", task.ID), fmt.Sprintf("%d_burned.mp4", task.ID))
	ytID, err := yt.UploadVideo(burnedPath, uploadTitle, description, "unlisted")
	if err != nil {
		log.Printf("[%d] Failed to upload burned to YouTube: %v", task.ID, err)
		d1.UpdateStatusBurned(task.ID, 2)
		return
	}

	err = d1.UpdateYTIDBurned(task.ID, ytID, 4)
	if err != nil {
		log.Printf("[%d] Failed to update status to uploaded: %v", task.ID, err)
	} else {
		notify.SendNotification(cfg, "YouTubeアップロード完了", fmt.Sprintf("VOD %d がYouTubeにアップロードされました。\nURL: https://youtu.be/%s", task.ID, ytID), "high")
	}
}
