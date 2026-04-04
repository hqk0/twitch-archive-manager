# Twitch Archive Manager (tam)

`tam` は、Twitch の配信録画（VOD）をアーカイブするための Go 製 CLI ツールです。録画のダウンロード、チャットの弾幕描画（ニコニコ動画風）、YouTube へのアップロードを簡単に行えます。

## 主な機能

- **高品質な弾幕描画**: Go による独自実装で、コメントの衝突や追い越しを防止した ASS 描画ファイルを生成。滑らかなチャット体験を動画上に再現します。

- **スタンドアロンモード**: `archive` コマンド一発で、ダウンロードからアップロードまで完結。複雑なデータベース設定は不要です。
- **ハードウェア加速**: macOS の `h264_videotoolbox` を活用し、超高速なエンコードを実現しています。
- **自由なカスタマイズ**: Go テンプレート形式で、YouTube のタイトルや概要欄を自分好みに設定可能。
- **高度な自動化**: クラウド（Cloudflare D1/R2）と連携し、リアルタイムロガーと組み合わせた大規模な自動アーカイブ運用にも対応しています。

## 必要条件

- [TwitchDownloaderCLI](https://github.com/lay295/TwitchDownloader)
- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [ffmpeg](https://ffmpeg.org/)
- **YouTube API 認証情報**:
  - `client_secret.json`: [Google Cloud Console](https://console.cloud.google.com/) から入手した OAuth 2.0 クライアント ID ファイル。
  - `youtube_token.json`: 認証済みトークンが保存されるファイル。

## 使い方

### 一括アーカイブ処理

ダウンロードから弾幕焼き込み、YouTube アップロードまでを一気に行います：

```bash
./tam archive <vod_id>
```

### ローカル保管・手動処理

YouTube にアップロードせず、手元の PC に弾幕付き動画を保存したい場合：

1. **録画のダウンロード**:

   ```bash
   ./tam download <vod_id>
   ```

   `./workspace/<vod_id>/<vod_id>.mp4` に保存されます。

2. **弾幕ファイルの生成**:

   ```bash
   ./tam ass <vod_id>
   ```

   Twitch からチャットを読み込み、`<vod_id>.ass` を生成します。動画と同じファイル名にするため、mpv 等のプレーヤーで再生するだけで自動的に弾幕が表示されます。

3. **弾幕の焼き込み (エンコード)**:
   ```bash
   ./tam burn <vod_id>
   ```
   動画に弾幕を直接合成し、`<vod_id>_burned.mp4` を作成します。

### 自動化ワーカー (上級者向け)

リアルタイムチャットロガーや Cloudflare D1 を使用している場合：

```bash
./tam worker
```

## 謝辞 (Acknowledgments)

本プロジェクトのチャット描画（ASS 変換）ロジックは、StarBrilliant 氏による [danmaku2ass](https://github.com/m13253/danmaku2ass) の実装を強く参考にし、Go 言語へ移植したものです。素晴らしいツールを開発されたコミュニティの貢献に深く感謝いたします。

## ライセンス

このプロジェクトは **GNU General Public License v3.0** の下で公開されています。詳細は `LICENSE` ファイルを参照してください。
