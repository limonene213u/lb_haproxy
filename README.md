# Goで作るHAProxyを使ったロードバランサー

このプロジェクトは、Go言語からHAProxy APIを操作して、以下の機能を提供するシンプルなロードバランサーです（翻訳：F5たっけぇからGoとHAProxyでロードバランサー作ってやると思ってやけっぱちでやった、公開はするし、後悔もしてない）。

- HAProxy APIへの接続確認を行う堅牢なクライアント初期化
- 設定ファイル（JSON形式）からバックエンドサーバーの情報、ヘルスチェック設定、再接続ポリシー（リトライ、redispatch）を読み込み
- バックエンドサーバーの追加時に、最大3回のリトライ処理を実施
- API経由でロードバランシングアルゴリズムや再接続ポリシーを適用

---

## 特徴

- **接続チェック付きのクライアント初期化**  
  HAProxy APIにPingリクエストを投げて接続が可能か確認してからクライアントを生成します。安全安心です、たぶん。

- **リトライ機能付きサーバー追加**  
  バックエンドサーバー追加時に最大3回リトライを試みます。というわけで、一時的なネットワーク障害などにも対応。

- **柔軟な設定管理**  
  JSON形式の設定ファイル (`config.json`) によって、バックエンドサーバー情報、ヘルスチェック、再接続ポリシーなどを一元管理。

- **APIによる設定反映**  
  ロードバランシングアルゴリズムや再接続ポリシー（retries, option redispatch）もHAProxy APIを通じて適用可能。

---

## 必要環境

- Go 1.16 以降
- HAProxy APIにアクセス可能な HAProxy サーバー
- [github.com/haproxytech/client-go/v2/haproxy](https://github.com/haproxytech/client-go) (Goライブラリ)

以下のコマンドで依存パッケージをインストールしてください。

```bash
go get github.com/haproxytech/client-go/v2/haproxy
```

---

## 設定

プロジェクトルートに `config.json` ファイルを作成し、以下の内容のように設定してください。config例はexample.config.jsonとして配布しています。

### config.json の例

```json
{
  "haproxy_endpoint": "http://localhost:9000",
  "api_key": "your_api_key_here",
  "load_balancing_algorithm": "roundrobin",
  "backends": [
    {
      "name": "server1",
      "ip": "192.168.1.101",
      "port": 80,
      "weight": 10
    },
    {
      "name": "server2",
      "ip": "192.168.1.102",
      "port": 80,
      "weight": 10
    }
  ],
  "health_check": {
    "enabled": true,
    "interval": 5,
    "fall": 3,
    "rise": 2
  },
  "retry_policy": {
    "retries": 3,
    "redispatch": true
  }
}
```

- **haproxy_endpoint**  
  HAProxy APIのエンドポイント（例：http://localhost:9000）

- **api_key**  
  APIキー（HAProxyの設定に合わせて適宜設定してください）

- **load_balancing_algorithm**  
  ロードバランシングアルゴリズム（例："roundrobin", "leastconn" など）

- **backends**  
  バックエンドサーバーの情報（ホスト名、IP、ポート、重みなど）

- **health_check**  
  ヘルスチェックの有効/無効、チェック間隔、失敗時・復帰時閾値の設定

- **retry_policy**  
  API経由で再接続ポリシーを設定するための、リトライ回数とredispatchオプション

---

## 使用方法

1. 依存パッケージのインストールおよびモジュール管理を行います。

   ```bash
   go mod tidy
   ```

2. プロジェクトをビルドします。

   ```bash
   go build -o haproxy-loadbalancer
   ```

3. 実行ファイルを起動します。

   ```bash
   ./haproxy-loadbalancer
   ```

実行すると、設定ファイルの内容に基づいて以下の動作が実施されます。

- HAProxy APIへ接続確認を行い、接続に成功するとクライアントを初期化
- 各バックエンドサーバーの追加を試み、リトライ処理を実施
- 指定したロードバランシングアルゴリズムおよび再接続ポリシーをHAProxy APIを通じて反映

---

## コード構成

- **main.go**  
  メインアプリケーションファイル。  
  - `newHAProxyClient(endpoint, apiKey string)`  
    HAProxy APIにPingリクエストを送信し、疎通確認を行います。
    
  - `addServerWithRetry(client *haproxy.HAProxy, server haproxy.Server, retries int)`  
    バックエンドサーバー追加時のリトライ処理を実装。
    
  - `setRetryPolicy(client *haproxy.HAProxy, rp RetryPolicyConfig)`  
    API経由で再接続ポリシー（retries, option redispatch）を設定。
    
  - `loadConfig(filename string)`  
    JSON形式の設定ファイルを読み込み、構造体にパース。

---

## 注意事項

- HAProxy APIのエンドポイントとAPIキーは実際の環境に合わせて設定してください。
- 本コードはシンプルなサンプル実装です。実運用時には、さらに詳細なエラーハンドリングや検証が必要になる場合がありますので、その辺はお好みで味変してどうぞ。

---

## ライセンス

このプロジェクトは MIT ライセンスの下で公開されています。

---

## 作者：りもこ

このプロジェクトは、GoとHAProxyを組み合わせたロードバランサーのサンプルとして作成されました。
