package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/haproxytech/client-go/v2/haproxy"
)

// Config はHAProxy接続情報、バックエンドサーバー設定に加え、
// ヘルスチェックおよび再接続ポリシーの設定を含みます
type Config struct {
	HaproxyEndpoint        string             `json:"haproxy_endpoint"`
	APIKey                 string             `json:"api_key"`
	LoadBalancingAlgorithm string             `json:"load_balancing_algorithm"`
	Backends               []BackendConfig    `json:"backends"`
	HealthCheck            HealthCheckConfig  `json:"health_check"`
	RetryPolicy            RetryPolicyConfig  `json:"retry_policy"`
}

// BackendConfig は各バックエンドサーバーの設定を表します
type BackendConfig struct {
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Weight int    `json:"weight"`
}

// HealthCheckConfig はヘルスチェックの設定値を保持します
type HealthCheckConfig struct {
	Enabled  bool `json:"enabled"`  // ヘルスチェックを有効にするかどうか
	Interval int  `json:"interval"` // チェック間隔（秒単位）
	Fall     int  `json:"fall"`     // 連続失敗回数の閾値
	Rise     int  `json:"rise"`     // 復帰と判断する連続成功回数
}

// RetryPolicyConfig は再接続（リトライ）ポリシーの設定を保持します
type RetryPolicyConfig struct {
	Retries    int  `json:"retries"`    // リトライ試行回数
	Redispatch bool `json:"redispatch"` // 別サーバーへの切り替え有無
}

func main() {
	// JSON形式の設定ファイルを読み込みます
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("設定ファイルの読み込みに失敗: %v", err)
	}

	// HAProxyクライアントの初期化（接続テスト付き）
	client, err := newHAProxyClient(config.HaproxyEndpoint, config.APIKey)
	if err != nil {
		log.Fatalf("HAProxyクライアントの初期化に失敗: %v", err)
	}

	// 設定ファイルに記載された各バックエンドサーバーを追加（リトライ付き）
	for _, backend := range config.Backends {
		server := haproxy.Server{
			Name:   backend.Name,
			IP:     backend.IP,
			Port:   backend.Port,
			Weight: int64(backend.Weight),
			Check:  config.HealthCheck.Enabled,
		}
		// ヘルスチェックが有効な場合のパラメータを設定
		if config.HealthCheck.Enabled {
			server.Inter = fmt.Sprintf("%ds", config.HealthCheck.Interval)
			server.Fall = config.HealthCheck.Fall
			server.Rise = config.HealthCheck.Rise
		}
		err := addServerWithRetry(client, server, 3)
		if err != nil {
			log.Printf("サーバー[%s]の追加に最終的に失敗: %v", backend.Name, err)
		}
	}

	// ロードバランシングアルゴリズムの設定
	err = client.SetLoadBalancingAlgorithm(config.LoadBalancingAlgorithm)
	if err != nil {
		log.Fatalf("ロードバランシングアルゴリズムの設定に失敗: %v", err)
	}
	fmt.Printf("ロードバランシングアルゴリズムを [%s] に設定しました\n", config.LoadBalancingAlgorithm)

	// 再接続ポリシー（リトライ設定と redispatch）の設定を反映
	err = setRetryPolicy(client, config.RetryPolicy)
	if err != nil {
		log.Fatalf("再接続ポリシーの設定に失敗: %v", err)
	}
}

// newHAProxyClient は、HAProxy APIにPingリクエストを送り接続できるか確認した上でクライアントを返します
func newHAProxyClient(endpoint, apiKey string) (*haproxy.HAProxy, error) {
	client := &haproxy.HAProxy{
		Endpoint: endpoint,
		ApiKey:   apiKey,
	}

	// 実際にPingでAPIの疎通確認を行う
	err := client.Ping()
	if err != nil {
		return nil, fmt.Errorf("HAProxy APIへの接続失敗: %w", err)
	}
	return client, nil
}

// addServerWithRetry は、サーバー追加処理を指定回数リトライします
func addServerWithRetry(client *haproxy.HAProxy, server haproxy.Server, retries int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = client.AddServer(&server)
		if err == nil {
			fmt.Printf("サーバー[%s]を正常に追加しました\n", server.Name)
			return nil
		}
		fmt.Printf("サーバー[%s]追加失敗 (試行 %d/%d): %v\n", server.Name, i+1, retries, err)
	}
	return fmt.Errorf("サーバー[%s]の追加に最終的に失敗しました: %w", server.Name, err)
}

// setRetryPolicy は、HAProxy APIを通じて再接続ポリシー（retries と option redispatch）を設定します
func setRetryPolicy(client *haproxy.HAProxy, rp RetryPolicyConfig) error {
	// retries の設定
	err := client.SetConfig("retries", fmt.Sprintf("%d", rp.Retries))
	if err != nil {
		return fmt.Errorf("再接続ポリシー（retries=%d）の設定失敗: %w", rp.Retries, err)
	}

	// redispatch の設定：有効なら "on", 無効なら "off" を指定
	var redispatchVal string
	if rp.Redispatch {
		redispatchVal = "on"
	} else {
		redispatchVal = "off"
	}
	err = client.SetConfig("option redispatch", redispatchVal)
	if err != nil {
		return fmt.Errorf("redispatchの設定失敗: %w", err)
	}

	fmt.Printf("再接続ポリシーを設定しました: retries=%d, redispatch=%v\n", rp.Retries, rp.Redispatch)
	return nil
}

// loadConfig は、指定されたJSON設定ファイルを読み込み Config 構造体へパースします
func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
