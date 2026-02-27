package db

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hqk0/twitch-archive-manager/internal/config"
)

type Video struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Category     string    `json:"category"`
	Duration     string    `json:"duration"`
	CreatedAt    time.Time `json:"created_at"`
	StatusRaw    int       `json:"status_raw"`
	StatusBurned int       `json:"status_burned"`
	YTID         string    `json:"yt_id"`
	YTIDBurned   string    `json:"yt_id_burned"`
}

type D1Client struct {
	Config *config.Config
}

func NewD1Client(cfg *config.Config) *D1Client {
	return &D1Client{Config: cfg}
}

type D1QueryResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		Results []map[string]interface{} `json:"results"`
	} `json:"result"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *D1Client) Query(sql string, params []interface{}) ([]map[string]interface{}, error) {
	endpoint := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/d1/database/%s/query",
		c.Config.CFAccountID, c.Config.D1DatabaseID)

	body := map[string]interface{}{
		"sql":    sql,
		"params": params,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+c.Config.CFAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("D1 API error: %s", string(respBody))
	}

	var d1Resp D1QueryResponse
	if err := json.Unmarshal(respBody, &d1Resp); err != nil {
		return nil, err
	}

	if !d1Resp.Success {
		msg := "Unknown error"
		if len(d1Resp.Errors) > 0 {
			msg = d1Resp.Errors[0].Message
		}
		return nil, fmt.Errorf("D1 error: %s", msg)
	}

	if len(d1Resp.Result) > 0 {
		return d1Resp.Result[0].Results, nil
	}

	return nil, nil
}

func (c *D1Client) GetAllVideos() ([]Video, error) {
	sql := "SELECT id, title, category, duration, created_at, status_raw, status_burned, yt_id, yt_id_burned FROM videos ORDER BY created_at DESC LIMIT 20;"
	return c.fetchVideos(sql, nil)
}

func (c *D1Client) GetPendingTasks() ([]Video, error) {
	// 焼き込みが完了していない(status_burned < 2)もの、
	// または、焼き込み完了だがアップロード未完了(status_burned == 2)のものを取得
	sql := "SELECT id, title, category, duration, created_at, status_raw, status_burned, yt_id, yt_id_burned FROM videos WHERE status_burned < 4;"
	return c.fetchVideos(sql, nil)
}

func (c *D1Client) fetchVideos(sql string, params []interface{}) ([]Video, error) {
	results, err := c.Query(sql, params)
	if err != nil {
		return nil, err
	}

	var videos []Video
	for _, r := range results {
		v := Video{
			ID:           int64(r["id"].(float64)),
			Title:        r["title"].(string),
			Category:     r["category"].(string),
			Duration:     r["duration"].(string),
			StatusRaw:    int(r["status_raw"].(float64)),
			StatusBurned: int(r["status_burned"].(float64)),
		}
		if ytId, ok := r["yt_id"].(string); ok {
			v.YTID = ytId
		}
		if ytIdBurned, ok := r["yt_id_burned"].(string); ok {
			v.YTIDBurned = ytIdBurned
		}
		
		if createdAt, ok := r["created_at"].(string); ok {
			t, _ := time.Parse(time.RFC3339, createdAt)
			v.CreatedAt = t
		}

		videos = append(videos, v)
	}

	return videos, nil
}

func logCreatedAtError(val string, err error) {
	// time.Parse失敗時のログ出力用（必要なら）
}

func (c *D1Client) UpdateStatusRaw(vodID int64, status int) error {
	sql := "UPDATE videos SET status_raw = ? WHERE id = ?;"
	_, err := c.Query(sql, []interface{}{status, vodID})
	return err
}

func (c *D1Client) UpdateStatusBurned(vodID int64, status int) error {
	sql := "UPDATE videos SET status_burned = ? WHERE id = ?;"
	_, err := c.Query(sql, []interface{}{status, vodID})
	return err
}

func (c *D1Client) UpdateYTIDBurned(vodID int64, ytID string, status int) error {
	sql := "UPDATE videos SET yt_id_burned = ?, status_burned = ? WHERE id = ?;"
	_, err := c.Query(sql, []interface{}{ytID, status, vodID})
	return err
}
