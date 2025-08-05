# **Splunk CLI Tool (splunk-cli)**

## **概要**

splunk-cliは、SplunkのREST APIと対話するための、Go言語で書かれたコマンドラインインターフェース（CLI）ツールです。SPL（Search Processing Language）クエリの実行、検索ジョブの管理、そして結果の取得を、ターミナルから直接、またはスクリプト経由で効率的に行うことができます。

### **解決する課題**

* **自動化**: シェルスクリプトやCI/CDジョブからSplunkの検索をトリガーし、結果を後続の処理に渡すことができます。  
* **効率性**: Web UIを開くことなく、コマンド一つで迅速にデータを確認できます。  
* **柔軟な認証**: コマンドライン引数、環境変数、設定ファイル、そして安全な対話プロンプトと、多様な方法で認証情報を管理できます。  
* **長時間のジョブ管理**: 非同期実行モデル（start, status, results）により、完了までに数時間かかるような重い検索ジョブも、ターミナルを占有することなく管理できます。  
* **Appコンテキスト**: --appフラグを使用することで、特定のAppコンテキストで検索を実行でき、Appに閉じたlookupなどを利用できます。

## **使い方**

### **設定**

最も便利な使い方は、設定ファイルを作成することです。

**パス**: ~/.config/splunk-cli/config.json

**内容例**:

{  
  "host": "https://your-splunk-instance.com:8089",  
  "token": "your-splunk-token-here",  
  "app": "search",  
  "insecure": true,  
  "httpTimeout": "60s"  
}

### **設定の優先順位**

設定は以下の優先順位で評価されます。強いものが優先されます。

1. **コマンドラインフラグ** (例: --host <URL>)  
2. **環境変数** (例: SPLUNK_HOST, SPLUNK_APP)  
3. **設定ファイル**

### **コマンド一覧**

#### **run**

検索を開始し、完了まで待って結果を表示します。

**使用例:**

# 過去1時間のデータを検索  
splunk-cli run --spl "index=_internal" --earliest "-1h"

# ファイルからSPLを読み込んで検索  
cat my_query.spl | splunk-cli run -f -

* **--spl <string>**: 実行するSPLクエリ。  
* **--file <path> / -f <path>**: SPLクエリをファイルから読み込みます。パスに-を指定すると標準入力から読み込みます。  
* **--earliest <time>**: 検索の開始時刻。(-1h, @d, 1672531200など)  
* **--latest <time>**: 検索の終了時刻。(now, @d, 1672617600など)  
* **--timeout <duration>**: ジョブ全体のタイムアウト時間。(10m, 1h30mなど)  
* **--silent**: 進捗メッセージを非表示にします。

💡 Ctrl+C の挙動  
runの実行中に Ctrl+C を押すと、ジョブをキャンセルするか、バックグラウンドで実行し続けるかを選択できます。

#### **start**

検索ジョブを開始し、ジョブID (SID) のみを標準出力に表示して即座に終了します。

**使用例:**

export JOB_ID=$(splunk-cli start --spl "index=main | stats count by sourcetype")  
echo "Job started with SID: $JOB_ID"

* --spl, --file, --earliest, --latestフラグが利用可能です。

#### **status**

指定したSIDのジョブの状態を確認します。

**使用例:**

splunk-cli status --sid "$JOB_ID"

* **--sid <string>**: 状態を確認したいジョブのSID。

#### **results**

完了したジョブの結果を取得します。jqと組み合わせて使うと便利です。

**使用例:**

splunk-cli results --sid "$JOB_ID" --silent | jq .

* **--sid <string>**: 結果を取得したいジョブのSID。

### **共通フラグ**

すべてのコマンド（または一部）で利用できるフラグです。

* --host <url>: SplunkサーバーのURL。  
* --token <string>: 認証トークン。  
* --user <string>: ユーザー名。  
* --password <string>: パスワード（指定しない場合はプロンプトで入力を求められます）。  
* --app <string>: 検索を実行するAppのコンテキスト。  
* --owner <string>: App内のナレッジオブジェクトの所有者。デフォルトはnobody。  
* --insecure: TLS証明書の検証をスキップします。  
* --http-timeout <duration>: 個々のAPIリクエストのタイムアウト時間。(30s, 1mなど)  
* --debug: 詳細なデバッグ情報を表示します。

## **謝辞**

このコードは、Googleの大規模言語モデルである**Gemini**との対話を通じて生成されました。