# CMPPChat

[![GitHub release](https://img.shields.io/github/v/release/puyokura/cmppchat)](https://github.com/puyokura/cmppchat/releases/latest)

コマンドライン上で動作するリアルタイムチャットアプリケーション。Go言語で開発され、WebSocketを使用した軽量なチャットシステムです。

## 特徴

- 🚀 **軽量・高速**: Go言語製のシングルバイナリ
- 💬 **リアルタイムチャット**: WebSocketによる低遅延通信
- 🏠 **完全ローカル**: 外部サービス不要、ホストPC上で完結
- 🎨 **TUI**: bubbletea/lipglossによる美しいターミナルUI
- 🔐 **ユーザー認証**: パスワードハッシュ化、管理者機能
- 🌐 **複数ルーム対応**: ルームごとのメッセージ管理

## ダウンロード

### 最新リリース

[Releases ページ](https://github.com/puyokura/cmppchat/releases)から最新版をダウンロードできます。

### 開発版 (Nightly Builds)

最新の開発版は以下からダウンロードできます：

| OS | アーキテクチャ | ダウンロード |
|---|---|---|
| Linux | x64 | [cmppchat-linux-amd64.zip](https://nightly.link/puyokura/cmppchat/workflows/release/main/cmppchat-linux-amd64.zip) |
| macOS | Intel (x64) | [cmppchat-darwin-amd64.zip](https://nightly.link/puyokura/cmppchat/workflows/release/main/cmppchat-darwin-amd64.zip) |
| macOS | Apple Silicon (ARM64) | [cmppchat-darwin-arm64.zip](https://nightly.link/puyokura/cmppchat/workflows/release/main/cmppchat-darwin-arm64.zip) |
| Windows | x64 | [cmppchat-windows-amd64.zip](https://nightly.link/puyokura/cmppchat/workflows/release/main/cmppchat-windows-amd64.zip) |

## クイックスタート

### 1. サーバーの初期化と起動

```bash
# 初回のみ: 設定ファイルとディレクトリを生成
./server init

# サーバーを起動
./server

# 外部からアクセス可能にする場合（ngrok等）
./server --http
```

サーバーはデフォルトで `localhost:8999` で起動します。

### 2. クライアントの起動

```bash
./client
```

起動後、以下のコマンドでサーバーに接続：

```
/connect localhost:8999
```

### 3. ユーザー登録とログイン

初回利用時はユーザー登録が必要です：

```
/register <ユーザー名> <パスワード>
```

2回目以降はログイン：

```
/login <ユーザー名> <パスワード>
```

## 基本コマンド

### ユーザーコマンド

| コマンド | 説明 |
|---------|------|
| `/register <user> <pass>` | 新規ユーザー登録 |
| `/login <user> <pass>` | ログイン |
| `/logout` | ログアウト |
| `/name <new_name>` | 表示名の変更 |
| `/connect <host:port>` | サーバーに接続 |
| `/disconnect` | サーバーから切断 |
| `/help` | ヘルプ表示 |

### ルーム管理コマンド

| コマンド | 説明 |
|---------|------|
| `/room join <room_name>` | ルームに参加 |
| `/room list` | 利用可能なルーム一覧 |
| `/room create <room_name>` | 新規ルーム作成（管理者のみ） |
| `/room remove <room_name>` | ルーム削除（管理者のみ） |

### 情報表示コマンド

| コマンド | 説明 |
|---------|------|
| `/member list [room]` | オンラインユーザー一覧 |
| `/userinfo <username>` | ユーザー詳細情報 |
| `/server info` | サーバー情報 |

### 管理者コマンド

| コマンド | 説明 |
|---------|------|
| `/admin <password>` | 管理者権限の取得 |
| `/kick <ip_id>` | ユーザーをキック |
| `/ban <ip_id>` | ユーザーをBAN |
| `/clan create <tag> <color>` | クラン作成 |
| `/clan add <tag> <username>` | クランにユーザー追加 |

## ビルド方法（開発者向け）

Go 1.21以上が必要です。

```bash
# サーバーとクライアントをビルド
go build -o server ./server
go build -o client ./client

# または test ディレクトリにビルド
go build -o ./test/server ./server && go build -o ./test/client ./client
```

## 設定ファイル

### server_config.json

サーバーの設定ファイル。`./server init` で自動生成されます。

```json
{
  "port": "8999",
  "host": "localhost",
  "admin_password": "admin",
  "welcome_message": "Welcome to CMPPChat!",
  "server_name": "CMPPChat Server",
  "rooms": ["general"]
}
```

### データファイル

- `users.json`: ユーザー情報（自動生成）
- `messages/<room_name>.json`: ルームごとのメッセージ履歴（自動生成）
- `logs/`: サーバーログ（自動生成、圧縮保存）

## 外部ホスティング（ngrok等）

`--http` フラグを使用すると、全インターフェース（0.0.0.0）でリッスンします：

```bash
./server --http
```

ngrokでポート転送：

```bash
ngrok http 8999
```

クライアントから接続：

```
/connect <ngrok-url>:8999
```

## セキュリティに関する注意

- パスワードはbcryptでハッシュ化されて保存されます
- 通信は平文WebSocketです（TLS未対応）
- 公共ネットワークでの使用には注意してください
- 管理者パスワードは `server_config.json` で変更可能です

## ライセンス

MIT License

## 開発

このプロジェクトはGo言語で開発されています。

主な依存ライブラリ：
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket通信
- [bubbletea](https://github.com/charmbracelet/bubbletea) - TUIフレームワーク
- [lipgloss](https://github.com/charmbracelet/lipgloss) - ターミナルスタイリング
