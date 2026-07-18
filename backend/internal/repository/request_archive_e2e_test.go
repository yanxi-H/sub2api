package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "modernc.org/sqlite"
)

// TestRequestArchiveE2E 端到端验证:建表 → 插入 → 查询 → 详情,确认 prompt 文本完整流转。
func TestRequestArchiveE2E(t *testing.T) {
	db, err := sql.Open("sqlite", "file:request_archive_e2e?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS request_archive_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			request_id TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			user_email TEXT,
			api_key_id INTEGER NOT NULL,
			api_key_name TEXT,
			group_id INTEGER,
			endpoint TEXT,
			protocol TEXT,
			model TEXT,
			ip_address TEXT,
			prompt_text TEXT NOT NULL DEFAULT '',
			truncated BOOLEAN NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	repo := NewRequestArchiveRepository(db)
	ctx := context.Background()

	entries := []service.RequestArchiveEntry{
		{
			RequestID: "req-test-001", UserID: 100, UserEmail: "test@example.com",
			APIKeyID: 200, APIKeyName: "test-key", Endpoint: "/v1/messages",
			Protocol: "anthropic_messages", Model: "claude-3-5-sonnet",
			IPAddress: "1.2.3.4", PromptText: "你好,请帮我写一段代码\n这是第二行",
		},
		{
			RequestID: "req-test-002", UserID: 101, UserEmail: "codex@example.com",
			APIKeyID: 201, APIKeyName: "codex-key", Endpoint: "/v1/responses",
			Protocol: "openai_responses", Model: "gpt-5",
			IPAddress: "5.6.7.8", PromptText: "Help me check the file contents",
		},
	}

	// 环节1: BatchInsert
	if err := repo.BatchInsert(ctx, entries); err != nil {
		t.Fatalf("BatchInsert failed: %v", err)
	}
	t.Log("PASS BatchInsert")

	// 环节2: 直接查 DB 确认 prompt_text 有内容(SQLite 不支持 LEFT/to_tsvector)
	rows, err := db.QueryContext(ctx, "SELECT id, prompt_text, user_email, protocol FROM request_archive_logs ORDER BY created_at DESC")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int64
		var prompt, email, protocol string
		if err := rows.Scan(&id, &prompt, &email, &protocol); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if prompt == "" {
			t.Errorf("record %d: prompt_text is EMPTY!", id)
		} else {
			t.Logf("PASS record %d [%s] %s: %s", id, protocol, email, truncate(prompt, 50))
		}
		count++
	}
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	// 环节3: GetByID 详情
	var detailID int64
	err = db.QueryRowContext(ctx, "SELECT id FROM request_archive_logs LIMIT 1").Scan(&detailID)
	if err != nil {
		t.Fatalf("GetByID prep failed: %v", err)
	}
	detail, err := repo.GetByID(ctx, detailID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if detail.PromptText == "" {
		t.Fatal("GetByID prompt_text 为空!")
	}
	t.Logf("PASS GetByID: %s", truncate(detail.PromptText, 80))
	t.Log("PASS PASS PASS 端到端数据流验证通过")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
