# Integration Tests

本目錄包含 API Server、notification worker、cart-recall worker 與 PostgreSQL 的整合測試。

## Principles

- migration 只建立 schema 與固定系統資料，例如 local admin、notification templates。
- user、product、cart、order、campaign 等 testcase data 必須透過 API 建立。
- 每個 testcase 使用唯一 email、name 或 UUID，不依賴測試執行順序。
- 優先透過 API 驗證結果；只有 API 無法觀察 outbox、inbox、receipt 或 idempotency 狀態時才能查詢 DB。
- asynchronous worker assertion 使用 polling + timeout，不使用固定 `time.Sleep`。
- Docker Compose lifecycle 由 Makefile 或 CI 管理；`TestMain` 只等待 API ready。

## Run

執行完整整合測試：

```bash
make integration-test
```

保留環境以便重複執行或除錯：

```bash
make integration-up
go test -tags=integration ./integration-tests/suites/...
make integration-down
```

執行單一 testcase：

```bash
go test -tags=integration ./integration-tests/suites/... -run TestCreateOrderFromCart -v
```

可透過 `INTEGRATION_API_URL` 覆寫 API URL；預設為 `http://localhost:18080`。

## Reset

本地環境需要完全重置時可執行：

```bash
./scripts/reset-test-db.sh
```

這個 script 會移除 integration Compose project 的 containers 與 volume，再由正式 migration 建立全新 database。一般 testcase 不會執行全域 reset。
