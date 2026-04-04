# Twitch Archive Manager (tam)

`tam` is a Go-based CLI tool for archiving Twitch VODs. It downloads broadcasts, renders chat as high-quality "Danmaku" (scrolling) overlays, and uploads them to YouTube.

## Key Features

- **High-Quality Chat Rendering**: Built-in Go logic to generate ASS files with collision and "chasing" prevention, delivering a smooth, NicoNico-style "Danmaku" experience.
- **Optional External SSD Support**: Smartly switches to an alternative workspace (`WORKSPACE_DIR`) if local disk space is low.
- **Standalone Mode**: Use the `archive` command to process everything from download to YouTube upload in one go, with no database required.
- **Hardware Acceleration**: Optimized for macOS using `h264_videotoolbox` for lightning-fast encoding.
- **Fully Customizable**: Use Go templates to personalize your YouTube titles and descriptions.
- **Advanced Automation**: The `worker` command (optional) integrates with Cloudflare D1/R2 for large-scale, automated archival workflows.

## Prerequisites

- [TwitchDownloaderCLI](https://github.com/lay295/TwitchDownloader)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [ffmpeg](https://ffmpeg.org/)
- **YouTube API Credentials**:
  - `client_secret.json`: Obtain this from the [Google Cloud Console](https://console.cloud.google.com/).
  - `youtube_token.json`: Stores your access tokens (generated automatically on first use).

## Usage

### One-shot Archiving

Download, render Danmaku, and upload to YouTube in one command:

```bash
./tam archive <vod_id>
```

### Local Storage & Manual Processing

For users who want to keep Danmaku-enabled videos locally without uploading to YouTube:

1. **Download VOD**:

   ```bash
   ./tam download <vod_id>
   ```

   Saves to `./workspace/<vod_id>/<vod_id>.mp4`.

2. **Generate Danmaku (ASS)**:

   ```bash
   ./tam ass <vod_id>
   ```

   Generates `<vod_id>.ass`. By matching the filename with the video, players like `mpv` will load the Danmaku automatically.

3. **Burn Danmaku to Video**:
   ```bash
   ./tam burn <vod_id>
   ```
   Creates a hard-coded video: `<vod_id>_burned.mp4`.

### Automated Workflow (For Advanced Users)

If you are using a real-time chat logger and Cloudflare D1:

```bash
./tam worker
```

## Acknowledgments

The chat rendering (ASS conversion) logic in this project is heavily inspired by and derived from [danmaku2ass](https://github.com/m13253/danmaku2ass) by StarBrilliant. We are grateful for their contribution to the community.

## License

This project is licensed under the **GNU General Public License v3.0**. See the `LICENSE` file for details.
