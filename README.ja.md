# Splunk CLIツール (splunk-cli)

**splunk-cli**は、SplunkのREST APIと対話するための、Go言語で書かれた強力かつ軽量なコマンドラインインターフェース（CLI）ツールです。ターミナルから直接、またはスクリプト経由で、SPL（Search Processing Language）クエリの実行、検索ジョブの管理、そして結果の取得を効率的に行うことができます。

## 主な機能

- **自動化**: シェルスクリプトやCI/CDジョブからSplunkの検索をトリガーし、結果を後続の処理に渡すことができます。
- **効率性**: Web UIを開くことなく、コマンド一つで迅速にデータを確認できます。
- **柔軟な認証**: コマンドライン引数、環境変数、設定ファイル、そして安全な対話プロンプトと、多様な方法で認証情報を管理できます。
- **長時間のジョブ管理**: 非同期実行モデル（`start`, `status`, `results`）により、完了までに数時間かかるような重い検索ジョブも、ターミナルを占有することなく管理できます。
- **Appコンテキスト**: `--app`フラグを使用することで、特定のAppコンテキストで検索を実行でき、Appに閉じたlookupなどを利用できます。

## インストール

`splunk-cli`のインストール方法は2つあります。

### 1. リリースから（推奨）

[GitHubリリースページ](https://github.com/magifd2/splunk-cli/releases)から、お使いのOS（macOS, Linux, Windows）に対応したコンパイル済みのバイナリをダウンロードできます。

### 2. ソースから

Goがインストールされている環境であれば、ソースコードからビルドすることも可能です。

```bash
# リポジトリをクローン
git clone https://github.com/magifd2/splunk-cli.git
cd splunk-cli

# ビルドを実行
make build

# 実行ファイルは dist/ ディレクトリに生成されます (例: dist/macos/splunk-cli)
```

## 使い方

### 設定

最も便利な使い方は、設定ファイルを作成することです。

**パス**: `~/.config/splunk-cli/config.json`

**内容例**:
```json
{
  "host": "https://your-splunk-instance.com:8089",
  "token": "your-splunk-token-here",
  "app": "search",
  "insecure": true,
  "httpTimeout": "60s"
}
```

### 設定の優先順位

設定は以下の優先順位で評価されます。強いものが優先されます。

1.  **コマンドラインフラグ (グローバル)** (例: `--config <path>`)
2.  **コマンドラインフラグ (コマンド固有)** (例: `--host <URL>`)
3.  **環境変数** (例: `SPLUNK_HOST`, `SPLUNK_APP`)
4.  **設定ファイル**

### グローバルフラグ

これらのフラグはどのコマンドでも使用できます:

- `--config <path>`: カスタム設定ファイルへのパス。デフォルトの `~/.config/splunk-cli/config.json` を上書きします。
- `--version`: バージョン情報を表示して終了します。

### コマンド一覧

`splunk-cli`は、タスクに応じたコマンドを提供します。

#### `run`

検索を開始し、完了まで待って結果を表示します。

**使用例**:
```bash
# 過去1時間のデータを検索
splunk-cli run --spl "index=_internal" --earliest "-1h"

# ファイルからSPLを読み込んで検索
cat my_query.spl | splunk-cli run -f -
```

- `--spl <string>`: 実行するSPLクエリ。
- `--file <path>` / `-f <path>`: SPLクエリをファイルから読み込みます。パスに`-`を指定すると標準入力から読み込みます。
- `--earliest <time>`: 検索の開始時刻。(-1h, @d, 1672531200など)
- `--latest <time>`: 検索の終了時刻。(now, @d, 1672617600など)
- `--timeout <duration>`: ジョブ全体のタイムアウト時間。(10m, 1h30mなど)
- `--silent`: 進捗メッセージを非表示にします。

> **💡 Ctrl+C の挙動**: `run`の実行中に `Ctrl+C` を押すと、ジョブをキャンセルするか、バックグラウンドで実行し続けるかを選択できます。

#### `start`

検索ジョブを開始し、ジョブID (SID) のみを標準出力に表示して即座に終了します。

**使用例**:
```bash
export JOB_ID=$(splunk-cli start --spl "index=main | stats count by sourcetype")
echo "Job started with SID: $JOB_ID"
```

#### `status`

指定したSIDのジョブの状態を確認します。

**使用例**:
```bash
splunk-cli status --sid "$JOB_ID"
```

#### `results`

完了したジョブの結果を取得します。`jq`のようなツールと組み合わせると便利です。

**使用例**:
```bash
splunk-cli results --sid "$JOB_ID" --silent | jq .
```

### 共通フラグ

ほとんどのコマンドで利用できる共通フラグです。

- `--host <url>`: SplunkサーバーのURL。
- `--token <string>`: 認証トークン。
- `--user <string>`: ユーザー名。
- `--password <string>`: パスワード（指定しない場合はプロンプトで入力を求められます）。
- `--app <string>`: 検索を実行するAppのコンテキスト。
- `--owner <string>`: App内のナレッジオブジェクトの所有者。デフォルトは`nobody`。
- `--insecure`: TLS証明書の検証をスキップします。
- `--http-timeout <duration>`: 個々のAPIリクエストのタイムアウト時間。(30s, 1mなど)
- `--debug`: 詳細なデバッグ情報を表示します。
- `--version`: バージョン情報を表示します。

## 開発

このプロジェクトは、共通の開発タスクに `Makefile` を使用しています。

- `make build`: 全てのターゲットプラットフォーム（macOS, Linux, Windows）のバイナリをビルドします。
- `make test`: テストを実行します。
- `make lint`: リンター（`golangci-lint`）を実行します。
- `make vulncheck`: 既知の脆弱性をスキャンします（`govulncheck`）。
- `make clean`: ビルド成果物を削除します。

## ライセンス

このプロジェクトは **MIT License** の下で公開されています。詳細は [LICENSE](LICENSE) ファイルをご覧ください。

---

*このツールは、Googleの大規模言語モデルであるGeminiとの対話を通じて初期構築および開発されました。*
