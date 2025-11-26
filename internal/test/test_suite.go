// Command-line stress test that simulates concurrent logins / refresh / logout
// operations against the API and produces CSV + HTML reports.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"redbook/config"

	"github.com/go-redis/redis/v8"
)

const baseURL = "http://127.0.0.1:8080/api/v1"

var client = &http.Client{Timeout: 10 * time.Second}

// 简单的 tokenPair
type tokenPair struct {
	Access  string
	Refresh string
}

// logoutResult 汇总 tryLogout 的行为，方便折叠到报告内。
type logoutResult struct {
	Device     string
	UsedToken  string // "access" or "refresh"
	LogoutCode int
	ErrMessage string
	Timestamp  time.Time
}

// ======================= 基本 HTTP helper =======================

// doPostJSON is a thin helper that serializes a JSON body and sends a POST request.
func doPostJSON(url string, body any, headers map[string]string) (int, []byte, error) {
	var buf []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		buf = b
	}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(buf))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, nil
}

// ======================= 登录 / 刷新 / 注销 Helpers =======================

// registerRaw issues a raw register request and returns status/data for assertions.
func registerRaw(mobile, username, password string) (int, []byte, error) {
	body := map[string]string{"mobile": mobile, "username": username, "password": password}
	return doPostJSON(baseURL+"/users/register", body, nil)
}

// registerUser ensures the test account exists (idempotent).
func registerUser(mobile, username, password string) error {
	status, _, err := registerRaw(mobile, username, password)
	if err != nil {
		return err
	}
	if status != 200 && status != 400 { // 400 表示已存在（可接受）
		return fmt.Errorf("register status %d", status)
	}
	return nil
}

// loginRaw executes a login request and returns status/data.
func loginRaw(username, password, device string) (int, []byte, error) {
	body := map[string]string{"username": username, "password": password}
	headers := map[string]string{"X-Device": device}
	return doPostJSON(baseURL+"/users/login", body, headers)
}

// loginUser simulates one device login and returns the issued tokens.
func loginUser(username, password, device string) (tokenPair, error) {
	status, data, err := loginRaw(username, password, device)
	if err != nil {
		return tokenPair{}, err
	}
	if status != 200 {
		return tokenPair{}, fmt.Errorf("login status %d body=%s", status, string(data))
	}
	var res map[string]string
	if err := json.Unmarshal(data, &res); err != nil {
		return tokenPair{}, err
	}
	return tokenPair{Access: res["access_token"], Refresh: res["refresh_token"]}, nil
}

// refreshRaw issues the refresh request and returns status/data.
func refreshRaw(refreshToken, device string) (int, []byte, error) {
	body := map[string]string{"refresh_token": refreshToken}
	headers := map[string]string{"X-Device": device}
	return doPostJSON(baseURL+"/users/refresh", body, headers)
}

// refreshTokenRequestStatus probes the refresh endpoint for verification.
func refreshTokenRequestStatus(refreshToken, device string) int {
	status, _, err := refreshRaw(refreshToken, device)
	if err != nil {
		return 0
	}
	return status
}

// rotateTokens calls refresh endpoint and returns the rotated pair.
func rotateTokens(refreshToken, device string) (tokenPair, error) {
	status, data, err := refreshRaw(refreshToken, device)
	if err != nil {
		return tokenPair{}, err
	}
	if status != 200 {
		return tokenPair{}, fmt.Errorf("refresh status %d body=%s", status, string(data))
	}
	var res map[string]string
	if err := json.Unmarshal(data, &res); err != nil {
		return tokenPair{}, err
	}
	return tokenPair{Access: res["access_token"], Refresh: res["refresh_token"]}, nil
}

// logoutOne posts /users/logout with the provided token.
func logoutOne(token, device string) (int, error) {
	req, _ := http.NewRequest("POST", baseURL+"/users/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Device", device)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = ioutil.ReadAll(resp.Body)
	return resp.StatusCode, nil
}

// tryLogout: 先尝试用 access 退出，若失败（401）则回退用 refresh 退出；返回 logoutResult
func tryLogout(accessToken, refreshToken, device string) logoutResult {
	// 1) 尝试用 access
	status, err := logoutOne(accessToken, device)
	if err != nil {
		return logoutResult{Device: device, UsedToken: "", LogoutCode: 0, ErrMessage: err.Error(), Timestamp: time.Now()}
	}
	if status == 200 {
		// 验证 refresh 被撤销（即 refresh 无法再刷新）
		rStatus := refreshTokenRequestStatus(refreshToken, device)
		if rStatus == 200 {
			return logoutResult{Device: device, UsedToken: "access", LogoutCode: 200, ErrMessage: "refresh still valid after logout", Timestamp: time.Now()}
		}
		return logoutResult{Device: device, UsedToken: "access", LogoutCode: 200, Timestamp: time.Now()}
	}

	// 2) access 失败 -> 退回 refresh
	status, err = logoutOne(refreshToken, device)
	if err != nil {
		return logoutResult{Device: device, UsedToken: "refresh", LogoutCode: 0, ErrMessage: err.Error(), Timestamp: time.Now()}
	}
	if status == 200 {
		// 验证 refresh 现在无效
		rStatus := refreshTokenRequestStatus(refreshToken, device)
		if rStatus == 200 {
			return logoutResult{Device: device, UsedToken: "refresh", LogoutCode: 200, ErrMessage: "refresh still valid after logout", Timestamp: time.Now()}
		}
		return logoutResult{Device: device, UsedToken: "refresh", LogoutCode: 200, Timestamp: time.Now()}
	}
	// 都失败
	return logoutResult{Device: device, UsedToken: "refresh", LogoutCode: status, Timestamp: time.Now()}
}

// ======================= 基础功能连通性测试 =======================

// endpointSmokeTests exercises register/login/refresh endpoints with positive and negative cases.
func endpointSmokeTests() error {
	username := fmt.Sprintf("smoke-%d", time.Now().UnixNano()%1000000)
	password := "SmokePwd123!"
	mobile := fmt.Sprintf("13%09d", time.Now().UnixNano()%1000000000)
	device := "smoke-device"

	// Fresh registration should succeed.
	if status, _, err := registerRaw(mobile, username, password); err != nil || status != http.StatusOK {
		return fmt.Errorf("register (new) failed: status=%d err=%v mobile=%s username=%s", status, err, mobile, username)
	}

	// Duplicate registration should be rejected (400).
	if status, _, err := registerRaw(mobile, username, password); err != nil || status != http.StatusBadRequest {
		return fmt.Errorf("register (duplicate) expected 400, mobile=%s username=%s got %d err=%v", mobile, username, status, err)
	}

	// Login success path.
	status, data, err := loginRaw(username, password, device)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("login (valid) failed: status=%d err=%v body=%s", status, err, string(data))
	}
	var loginResp map[string]string
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return fmt.Errorf("login response decode failed: %w", err)
	}

	// Login with wrong password should be unauthorized.
	if status, _, err := loginRaw(username, "wrong-password", device); err != nil || status != http.StatusUnauthorized {
		return fmt.Errorf("login (invalid creds) expected 401, got %d err=%v", status, err)
	}

	// Refresh rotation succeeds.
	status, _, err = refreshRaw(loginResp["refresh_token"], device)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("refresh (valid) failed: status=%d err=%v", status, err)
	}

	// Refresh with mismatched device should be rejected.
	if status, _, err := refreshRaw(loginResp["refresh_token"], "other-device"); err != nil || status != http.StatusUnauthorized {
		return fmt.Errorf("refresh (device mismatch) expected 401, got %d err=%v", status, err)
	}

	log.Println("endpoint smoke tests passed: register/login/refresh basic scenarios verified")
	return nil
}

func sanityFlowTest(mobile, username, password string) error {
	device := "sanity-device"
	if err := registerUser(mobile, username, password); err != nil {
		return fmt.Errorf("sanity register failed: %w", err)
	}

	tokens, err := loginUser(username, password, device)
	if err != nil {
		return fmt.Errorf("sanity login failed: %w", err)
	}

	rotated, err := rotateTokens(tokens.Refresh, device)
	if err != nil {
		return fmt.Errorf("sanity refresh failed: %w", err)
	}

	status, err := logoutOne(rotated.Access, device)
	if err != nil || status != 200 {
		return fmt.Errorf("sanity logout failed: status=%d err=%v", status, err)
	}
	return nil
}

// ======================= 并发测试与报告生成 =======================

// concurrentLogoutTest orchestrates the whole test run (login -> logout -> report).
func concurrentLogoutTest(mobile, username, password string, devices []string, maxConcurrent int, outCSV, outHTML string) error {
	// 初始化 config + Redis 清理（可选）
	config.InitConfig("../../")
	config.InitRedis()
	rdb := config.RedisClient
	// 清空测试DB以避免脏数据
	_ = rdb.FlushDB(rdb.Context())

	// 确保测试用户存在
	if err := registerUser(mobile, username, password); err != nil {
		return fmt.Errorf("register error: %v", err)
	}

	type jobResult struct {
		Device string
		Pair   tokenPair
		Err    error
	}

	// 1) 并发登录（先获取每台设备的 token）
	jobs := make(chan string, len(devices))
	results := make(chan jobResult, len(devices))

	var wg sync.WaitGroup
	worker := func() {
		for d := range jobs {
			p, err := loginUser(username, password, d)
			results <- jobResult{Device: d, Pair: p, Err: err}
		}
		wg.Done()
	}

	workers := maxConcurrent
	if workers < 1 {
		workers = 10
	}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	for _, d := range devices {
		jobs <- d
	}
	close(jobs)
	wg.Wait()
	close(results)

	// Collect successful logins
	tokenMap := make(map[string]tokenPair) // device -> tokenPair
	for res := range results {
		if res.Err != nil {
			log.Printf("[login error] device=%s err=%v\n", res.Device, res.Err)
			continue
		}
		tokenMap[res.Device] = res.Pair
	}

	// 2) 并发执行 tryLogout（使用各自 token pair）
	var outWg sync.WaitGroup
	resCh := make(chan logoutResult, len(tokenMap))

	for d, pair := range tokenMap {
		outWg.Add(1)
		go func(device string, p tokenPair) {
			defer outWg.Done()
			// 模拟 Access 过期场景（可配置）：在这里先 sleep 以便 access 可能过期
			// time.Sleep(3 * time.Second) // 如果你将 Access TTL 很短可启用
			res := tryLogout(p.Access, p.Refresh, device)
			resCh <- res
		}(d, pair)
	}
	outWg.Wait()
	close(resCh)

	// 3) 写 CSV 报告
	csvFile, err := os.Create(outCSV)
	if err != nil {
		return err
	}
	defer csvFile.Close()
	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()
	_ = csvWriter.Write([]string{"Device", "UsedToken", "LogoutCode", "ErrMessage", "Timestamp"})

	var allResults []logoutResult
	for r := range resCh {
		_ = csvWriter.Write([]string{r.Device, r.UsedToken, fmt.Sprintf("%d", r.LogoutCode), r.ErrMessage, r.Timestamp.Format(time.RFC3339)})
		allResults = append(allResults, r)
	}
	csvWriter.Flush()

	// 4) 生成简单 HTML 报告
	if err := writeHTMLReport(outHTML, allResults); err != nil {
		log.Printf("write HTML report error: %v", err)
	}

	// 5) 打印 Redis keys（可选）
	keys, _ := rdb.Keys(rdb.Context(), "rb:*").Result()
	log.Printf("Redis keys after test: %v\n", keys)

	return nil
}

// writeHTMLReport renders a basic table so failures are easy to eyeball.
func writeHTMLReport(path string, results []logoutResult) error {
	const tpl = `
<!doctype html>
<html>
<head><meta charset="utf-8"><title>Logout Test Report</title>
<style>
table { border-collapse: collapse; width: 100%; }
th, td { border: 1px solid #ddd; padding: 8px; text-align:left }
th { background: #f4f4f4; }
.success { color: green; }
.fail { color: red; }
</style>
</head>
<body>
<h2>Logout Test Report ({{ .GeneratedAt }})</h2>
<table>
<thead><tr><th>Device</th><th>UsedToken</th><th>LogoutCode</th><th>Error</th><th>Timestamp</th></tr></thead>
<tbody>
{{ range .Rows }}
<tr>
<td>{{ .Device }}</td>
<td>{{ .UsedToken }}</td>
<td>{{ .LogoutCode }}</td>
<td>{{ .ErrMessage }}</td>
<td>{{ .Timestamp }}</td>
</tr>
{{ end }}
</tbody>
</table>
</body>
</html>`

	data := struct {
		GeneratedAt string
		Rows        []logoutResult
	}{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Rows:        results,
	}

	t, err := template.New("report").Parse(tpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, data)
}

// ======================= main =======================

func main() {
	rdb := initRedis()
	// 配置（调整为你需要的并发和设备数）
	username := fmt.Sprintf("smoke-%d", time.Now().UnixNano()%1000000)
	password := "SmokePwd123!"
	mobile := fmt.Sprintf("13%09d", time.Now().UnixNano()%1000000000)
	deviceCount := 5   // 模拟设备数量
	maxConcurrent := 5 // 最大并发登录 worker 数
	outCSV := "logout_report.csv"
	outHTML := "logout_report.html"

	// 生成 device 列表
	devices := make([]string, 0, deviceCount)
	for i := 0; i < deviceCount; i++ {
		devices = append(devices, fmt.Sprintf("device-%d", i))
	}

	if err := endpointSmokeTests(); err != nil {
		log.Fatalf("endpoint smoke tests failed: %v", err)
	}
	if err := sanityFlowTest(mobile, username, password); err != nil {
		log.Fatalf("basic flow verification failed: %v", err)
	}

	start := time.Now()
	if err := concurrentLogoutTest(mobile, username, password, devices, maxConcurrent, outCSV, outHTML); err != nil {
		log.Fatalf("concurrent test failed: %v", err)
	}
	elapsed := time.Since(start)
	log.Printf("concurrent test finished in %s, CSV=%s HTML=%s\n", elapsed.String(), outCSV, outHTML)
	// 打印 Redis 状态
	keys, _ := rdb.Keys(rdb.Context(), "rb:*").Result()
	log.Printf("Redis keys after test: %v\n", keys)
	fmt.Println("All multi-device tests completed successfully!")
}

// 初始化 Redis 并清理测试数据
func initRedis() *redis.Client {
	config.InitConfig("../../")
	config.InitRedis()
	rdb := config.RedisClient
	rdb.FlushDB(rdb.Context())
	fmt.Println("Redis cleared for testing")
	return rdb
}
