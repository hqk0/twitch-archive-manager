package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var ErrTokenInvalid = errors.New("youtube token invalid or expired")

type YouTubeClient struct {
	Service   *youtube.Service
	TokenPath string
}

func NewYouTubeClient(ctx context.Context, clientSecretPath, tokenPath string) (*YouTubeClient, error) {
	b, err := os.ReadFile(clientSecretPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeUploadScope, youtube.YoutubeForceSslScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	client := getClient(ctx, config, tokenPath)
	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create YouTube service: %v", err)
	}

	return &YouTubeClient{Service: service, TokenPath: tokenPath}, nil
}

func getClient(ctx context.Context, config *oauth2.Config, tokenPath string) *http.Client {
	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokenPath, tok)
	}
	return config.Client(ctx, tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	codeChan := make(chan string)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to bind to a local port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	config.RedirectURL = fmt.Sprintf("http://localhost:%d", port)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			fmt.Fprintf(w, "Authentication successful! You can close this window.")
			go func() {
				codeChan <- code
			}()
		} else {
			fmt.Fprintf(w, "Authorization code not found.")
		}
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start local server: %v", err)
		}
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Authorize application here: \n%v\n", authURL)

	authCode := <-codeChan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func (c *YouTubeClient) UploadVideo(filePath, title, description, privacyStatus string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	stat, _ := file.Stat()
	fileSize := stat.Size()

	video := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: description,
			CategoryId:  "24",
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus:           privacyStatus,
			SelfDeclaredMadeForKids: false,
			ForceSendFields:         []string{"SelfDeclaredMadeForKids"},
		},
	}

	call := c.Service.Videos.Insert([]string{"snippet", "status"}, video)

	bar := progressbar.NewOptions64(
		fileSize,
		progressbar.OptionSetDescription(fmt.Sprintf("Uploading %s", title)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	reader := &progressReader{
		file: file,
		bar:  bar,
	}

	response, err := call.Media(reader).Do()
	if err != nil {
		if strings.Contains(err.Error(), "invalid_grant") || strings.Contains(err.Error(), "Token has been expired") {
			if c.TokenPath != "" {
				os.Remove(c.TokenPath)
				log.Printf("Token error detected. Removed token file: %s. Please re-run the program to authenticate again.", c.TokenPath)
			}
			return "", ErrTokenInvalid
		}
		return "", fmt.Errorf("error uploading video: %v", err)
	}

	return response.Id, nil
}

type progressReader struct {
	file *os.File
	bar  *progressbar.ProgressBar
}

func (r *progressReader) Read(p []byte) (n int, err error) {
	n, err = r.file.Read(p)
	if n > 0 {
		r.bar.Add(n)
	}
	return n, err
}

func (c *YouTubeClient) SetPrivacyStatus(videoID, privacyStatus string) error {
	// Fetch the existing video first to preserve other status fields like Embeddable
	call := c.Service.Videos.List([]string{"status"}).Id(videoID)
	response, err := call.Do()
	if err != nil {
		return fmt.Errorf("error fetching video status: %v", err)
	}

	if len(response.Items) == 0 {
		return fmt.Errorf("video not found: %s", videoID)
	}

	video := response.Items[0]
	video.Status.PrivacyStatus = privacyStatus

	_, err = c.Service.Videos.Update([]string{"status"}, video).Do()
	return err
}
