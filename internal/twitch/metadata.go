package twitch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/hqk0/twitch-archive-manager/internal/config"
)

type Metadata struct {
	VODID           int64
	Title           string
	DurationSeconds int
	DurationHMS     string
	CreatedAtUTC    string
	CreatedAtLocal  string
	CreatedAtDate   string // yyyy/mm/dd
	CategoriesStr   string
	Chapters        []Chapter
	HasMultipleCategories bool
	ViewerURL       string
}

type Chapter struct {
	Timestamp string
	Title     string
}

func GetVODMetadata(vodID int64) (*Metadata, error) {
	cmd := exec.Command("TwitchDownloaderCLI", "info", "-u", fmt.Sprintf("%d", vodID), "-f", "raw")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("TwitchDownloaderCLI error: %v, output: %s", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	var jsonBlocks []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			jsonBlocks = append(jsonBlocks, trimmed)
		}
	}

	if len(jsonBlocks) < 2 {
		return nil, fmt.Errorf("failed to find enough JSON blocks in output")
	}

	var mainData struct {
		Data struct {
			Video struct {
				Title         string `json:"title"`
				LengthSeconds int    `json:"lengthSeconds"`
				CreatedAt     string `json:"createdAt"`
				Game          struct {
					DisplayName string `json:"displayName"`
				} `json:"game"`
			} `json:"video"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonBlocks[0]), &mainData); err != nil {
		return nil, err
	}

	var momentsData struct {
		Data struct {
			Video struct {
				Moments struct {
					Edges []struct {
						Node struct {
							PositionMilliseconds int    `json:"positionMilliseconds"`
							Description         string `json:"description"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"moments"`
			} `json:"video"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonBlocks[1]), &momentsData); err != nil {
		return nil, err
	}

	jst := time.FixedZone("JST", 9*60*60)
	t, _ := time.Parse(time.RFC3339, mainData.Data.Video.CreatedAt)
	tJST := t.In(jst)

	metadata := &Metadata{
		VODID:           vodID,
		Title:           mainData.Data.Video.Title,
		DurationSeconds: mainData.Data.Video.LengthSeconds,
		DurationHMS:     formatDuration(mainData.Data.Video.LengthSeconds),
		CreatedAtUTC:    mainData.Data.Video.CreatedAt,
		CreatedAtLocal:  tJST.Format("2006年1月2日 15時04分05秒"),
		CreatedAtDate:   tJST.Format("2006/01/02"),
	}

	var categories []string
	if mainData.Data.Video.Game.DisplayName != "" {
		game := convertCategory(mainData.Data.Video.Game.DisplayName)
		categories = append(categories, game)
	}

	for _, edge := range momentsData.Data.Video.Moments.Edges {
		node := edge.Node
		posS := node.PositionMilliseconds / 1000
		h := posS / 3600
		m := (posS % 3600) / 60
		s := posS % 60
		ts := fmt.Sprintf("%d:%02d:%02d", h, m, s)
		
		game := convertCategory(node.Description)
		metadata.Chapters = append(metadata.Chapters, Chapter{Timestamp: ts, Title: game})
		
		found := false
		for _, cat := range categories {
			if cat == game {
				found = true
				break
			}
		}
		if !found {
			categories = append(categories, game)
		}
	}
	metadata.CategoriesStr = strings.Join(categories, " / ")
	metadata.HasMultipleCategories = len(categories) > 1

	return metadata, nil
}

func formatDuration(totalSeconds int) string {
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	res := ""
	if h > 0 {
		res += fmt.Sprintf("%d時間", h)
	}
	if m > 0 {
		res += fmt.Sprintf("%d分", m)
	}
	if s > 0 || res == "" {
		res += fmt.Sprintf("%d秒", s)
	}
	return res
}

func convertCategory(name string) string {
	if name == "Just Chatting" {
		return "雑談"
	}
	return name
}

func GenerateDescription(metadata *Metadata, cfg *config.Config) string {
	// Add Viewer URL if configured
	if cfg.ViewerBaseURL != "" {
		metadata.ViewerURL = fmt.Sprintf("%s?v=%d", strings.TrimSuffix(cfg.ViewerBaseURL, "/"), metadata.VODID)
	}

	templateContent := ""
	if b, err := os.ReadFile(cfg.YTDescTemplateFile); err == nil {
		templateContent = string(b)
	} else {
		// Default generic template if file not found
		templateContent = `{{if .HasMultipleCategories}}【カテゴリチャプター】
{{range .Chapters}}{{.Timestamp}} {{.Title}}
{{end}}
{{end}}
【配信詳細】
カテゴリ: {{.CategoriesStr}}
配信時間: {{.DurationHMS}}
配信日時: {{.CreatedAtLocal}}
{{if .ViewerURL}}
Twitchスタイルで視聴: {{.ViewerURL}}
{{end}}`
	}

	tmpl, err := template.New("desc").Parse(templateContent)
	if err != nil {
		return "Error parsing template: " + err.Error()
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, metadata); err != nil {
		return "Error executing template: " + err.Error()
	}

	return buf.String()
}

func GenerateUploadTitle(metadata *Metadata, cfg *config.Config) string {
	t := cfg.YTTitleTemplate
	t = strings.ReplaceAll(t, "%title%", metadata.Title)
	t = strings.ReplaceAll(t, "%date%", metadata.CreatedAtDate)
	return t
}

func ParseTwitchDuration(durationStr string) (time.Duration, error) {
	re := regexp.MustCompile(`(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?`)
	matches := re.FindStringSubmatch(durationStr)
	
	var d time.Duration
	if len(matches) > 1 && matches[1] != "" {
		val, _ := time.ParseDuration(matches[1] + "h")
		d += val
	}
	if len(matches) > 2 && matches[2] != "" {
		val, _ := time.ParseDuration(matches[2] + "m")
		d += val
	}
	if len(matches) > 3 && matches[3] != "" {
		val, _ := time.ParseDuration(matches[3] + "s")
		d += val
	}
	
	return d, nil
}

type IntegratedComment struct {
	Vpos      int    `json:"vpos"`
	Timestamp int    `json:"timestamp"`
	Author    string `json:"author"`
	Message   string `json:"message"`
}

func DownloadChatJSON(vodID int64, outputPath string) error {
	cmd := exec.Command("TwitchDownloaderCLI", "chatdownload", "-u", fmt.Sprintf("%d", vodID), "-o", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TwitchDownloaderCLI chatdownload error: %v, output: %s", err, string(output))
	}
	return nil
}

func ConvertTwitchJSONToIntegratedJSON(inputPath, outputPath string) error {
	b, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	var twitchData struct {
		Comments []struct {
			ContentOffsetSeconds float64 `json:"content_offset_seconds"`
			Commenter            struct {
				DisplayName string `json:"display_name"`
			} `json:"commenter"`
			Message struct {
				Body string `json:"body"`
			} `json:"message"`
		} `json:"comments"`
	}

	if err := json.Unmarshal(b, &twitchData); err != nil {
		return err
	}

	var integrated []IntegratedComment
	for _, c := range twitchData.Comments {
		ts := int(c.ContentOffsetSeconds)
		integrated = append(integrated, IntegratedComment{
			Vpos:      ts * 100, // Approximate vpos
			Timestamp: ts,
			Author:    c.Commenter.DisplayName,
			Message:   c.Message.Body,
		})
	}

	outB, _ := json.MarshalIndent(integrated, "", "  ")
	return os.WriteFile(outputPath, outB, 0644)
}
