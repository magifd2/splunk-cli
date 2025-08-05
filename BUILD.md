# **splunk-cliのビルド方法**

このドキュメントでは、splunk-cliツールをソースコードからビルドするための手順を説明します。

## **1\. 前提条件**

* **Go言語**: バージョン1.18以上がインストールされている必要があります。  
* **Git**: ソースコードのクローンに必要です（任意）。

## **2\. ビルド手順**

### **ステップ 1: ソースコードの準備**

splunk-cli.goとbuild.shを同じディレクトリに保存します。

### **ステップ 2: 依存関係の取得**

ターミナルで以下のコマンドを実行し、必要な外部パッケージをダウンロードします。

go mod init splunk-cli  
go mod tidy

### **ステップ 3: ビルドスクリプトの実行**

ビルドを簡単にするため、build.shスクリプトを用意しています。このスクリプトは、macOS、Windows、Linux向けの実行可能ファイルを一度に生成します。

まず、スクリプトに実行権限を付与します。

chmod \+x build.sh

次に、スクリプトを実行します。

./build.sh

スクリプトが正常に完了すると、プロジェクトルートにdistディレクトリが作成され、その中に各OS向けの実行可能ファイルが格納されます。

* dist/macos/splunk-cli (macOS向けユニバーサルバイナリ)  
* dist/windows/splunk-cli.exe (Windows向け)  
* dist/linux/splunk-cli (Linux向け)

## **(参考) 手動でのビルド**

ビルドスクリプトを使わずに、手動で特定のOS向けにビルドすることも可能です。

### **ローカル環境向けのビルド**

現在使用しているOS向けの実行可能ファイルを作成します。

go build \-o splunk-cli splunk-cli.go

### **クロスコンパイル (他のOS向けのビルド)**

Goのクロスコンパイル機能を利用して、他のOS向けのバイナリを生成できます。

#### **Windows (amd64) 向け**

GOOS=windows GOARCH=amd64 go build \-o splunk-cli.exe splunk-cli.go

#### **Linux (amd64) 向け**

GOOS=linux GOARCH=amd64 go build \-o splunk-cli splunk-cli.go

#### **macOS (ユニバーサルバイナリ)**

Apple Silicon (arm64)とIntel (amd64)の両方で動作する単一のバイナリを作成します。

\# arm64向けにビルド  
GOOS=darwin GOARCH=arm64 go build \-o splunk-cli-arm64 splunk-cli.go

\# amd64向けにビルド  
GOOS=darwin GOARCH=amd64 go build \-o splunk-cli-amd64 splunk-cli.go

\# lipoコマンドで2つを結合  
lipo \-create \-output splunk-cli splunk-cli-arm64 splunk-cli-amd64

\# 一時ファイルを削除  
rm splunk-cli-arm64 splunk-cli-amd64  
