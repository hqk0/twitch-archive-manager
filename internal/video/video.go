package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func DownloadVOD(vodID int64, outputDir string) (string, error) {
	outputPattern := filepath.Join(outputDir, fmt.Sprintf("%d.mp4", vodID))
	
	// Check if already exists
	if _, err := os.Stat(outputPattern); err == nil {
		return outputPattern, nil
	}

	// Using best available mp4 format
	cmd := exec.Command("yt-dlp", 
		"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best",
		"--merge-output-format", "mp4",
		"-o", outputPattern,
		fmt.Sprintf("https://www.twitch.tv/videos/%d", vodID),
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	return outputPattern, nil
}

func BurnSubtitles(videoPath, assPath, outputPath string) error {
	// Use h264_videotoolbox for hardware acceleration on MacBook
	// Fallback to libx264 is possible but we assume Videotoolbox is available
	
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vf", fmt.Sprintf("ass=%s", assPath),
		"-c:v", "h264_videotoolbox",
		"-q:v", "48", // Matches render.sh
		"-r", "60",   // Matches render.sh
		"-movflags", "+faststart",
		"-c:a", "copy",
		"-y",
		outputPath,
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err := cmd.Run()
	if err != nil {
		// Fallback to software encoding if Videotoolbox fails
		fmt.Printf("Hardware encoding failed, falling back to libx264: %v\n", err)
		cmdFallback := exec.Command("ffmpeg",
			"-i", videoPath,
			"-vf", fmt.Sprintf("ass=%s", assPath),
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "23",
			"-c:a", "copy",
			"-y",
			outputPath,
		)
		cmdFallback.Stdout = os.Stdout
		cmdFallback.Stderr = os.Stderr
		return cmdFallback.Run()
	}
	
	return nil
}
