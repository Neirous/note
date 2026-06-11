package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"note/internal/api"
	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"
)

type CheckResult struct {
	Step string
	OK   bool
	Msg  string
}

func Run(dsn string, cleanup bool) ([]CheckResult, error) {
	results := make([]CheckResult, 0, 16)

	dbFile := sqliteFileFromDSN(dsn)
	if cleanup && dbFile != "" {
		_ = os.Remove(dbFile)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	if cleanup && dbFile != "" {
		defer os.Remove(dbFile)
	}

	st := store.New(db)
	if err := st.InitSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return results, fmt.Errorf("OPENAI_API_KEY is required for e2e check")
	}
	baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
	chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")

	client := llm.NewOpenAICompatibleClient(&http.Client{Timeout: 90 * time.Second}, baseURL, apiKey, embedModel, chatModel)
	ragService := rag.NewService(st, client, client, rag.Config{MaxChunkChars: 800, TopK: 5})
	srv := api.NewServer(st, ragService)
	handler := srv.Routes()

	noteBodies := []map[string]any{
		{"title": "旅行计划 2026", "markdown": "# 旅行计划 2026\n\n目标：春季完成一次 5 天城市漫游\n\n## TODO\n- [ ] 订机票\n- [x] 预估预算\n\n## 预算表\n| 项目 | 金额 |\n| --- | --- |\n| 交通 | 3200 |\n| 住宿 | 2600 |", "tags": []string{"travel", "plan"}},
		{"title": "Go 学习笔记", "markdown": "# Go 学习笔记\n\n## Slice\n- append 可能触发扩容\n- 传参要关注底层数组共享\n\n## 示例\n```go\nfunc sum(nums []int) int {\n  s := 0\n  for _, n := range nums { s += n }\n  return s\n}\n```", "tags": []string{"go", "backend"}},
		{"title": "阅读清单", "markdown": "# 阅读清单\n\n1. 《Designing Data-Intensive Applications》\n2. 《Clean Architecture》\n\n> 每周至少读 2 章并输出摘要", "tags": []string{"reading"}},
	}

	var noteIDs []int64
	for i, body := range noteBodies {
		code, raw := requestJSON(handler, http.MethodPost, "/api/notes", body)
		if code != http.StatusCreated {
			results = append(results, CheckResult{Step: fmt.Sprintf("create_note_%d", i+1), OK: false, Msg: fmt.Sprintf("status=%d", code)})
			continue
		}
		note, warn, parseErr := parseNoteResponse(raw)
		if parseErr != nil {
			results = append(results, CheckResult{Step: fmt.Sprintf("create_note_%d", i+1), OK: false, Msg: parseErr.Error()})
			continue
		}
		noteIDs = append(noteIDs, note.ID)
		msg := fmt.Sprintf("id=%d title=%q", note.ID, note.Title)
		if warn != "" {
			msg += " index_warning=" + warn
		}
		results = append(results, CheckResult{Step: fmt.Sprintf("create_note_%d", i+1), OK: true, Msg: msg})
	}

	// List notes
	code, raw := requestJSON(handler, http.MethodGet, "/api/notes", nil)
	if code == http.StatusOK {
		var listed []store.Note
		if err := json.Unmarshal(raw, &listed); err == nil {
			results = append(results, CheckResult{Step: "list_notes", OK: len(listed) >= 3, Msg: fmt.Sprintf("count=%d", len(listed))})
		}
	}

	// Query notes
	code, raw = requestJSON(handler, http.MethodGet, "/api/notes?q=Go", nil)
	if code == http.StatusOK {
		var listed []store.Note
		json.Unmarshal(raw, &listed)
		ok := false
		for _, n := range listed {
			if strings.Contains(n.Title, "Go") {
				ok = true
				break
			}
		}
		results = append(results, CheckResult{Step: "query_notes", OK: ok, Msg: fmt.Sprintf("matched=%d", len(listed))})
	}

	// Update note
	if len(noteIDs) >= 2 {
		path := fmt.Sprintf("/api/notes/%d", noteIDs[1])
		code, raw = requestJSON(handler, http.MethodPut, path, map[string]any{
			"title": "Go 学习笔记（已更新）", "markdown": "# Go 学习笔记（已更新）\n\n## 今日重点\n- map 并发读写要加锁\n- context 用于超时与取消", "tags": []string{"go", "backend", "updated"},
		})
		if code == http.StatusOK {
			n, _, _ := parseNoteResponse(raw)
			results = append(results, CheckResult{Step: "update_note", OK: strings.Contains(n.Title, "已更新"), Msg: fmt.Sprintf("title=%q", n.Title)})
		}
	}

	// Delete note
	if len(noteIDs) >= 3 {
		path := fmt.Sprintf("/api/notes/%d", noteIDs[2])
		code, _ = requestJSON(handler, http.MethodDelete, path, nil)
		results = append(results, CheckResult{Step: "delete_note", OK: code == http.StatusOK, Msg: fmt.Sprintf("status=%d", code)})
	}

	// RAG search
	code, raw = requestJSON(handler, http.MethodPost, "/api/rag/search", map[string]any{"query": "Go 并发读写和 context 的重点", "top_k": 3})
	if code == http.StatusOK {
		var rs rag.SearchResult
		if err := json.Unmarshal(raw, &rs); err == nil {
			results = append(results, CheckResult{Step: "rag_search", OK: len(rs.Results) > 0, Msg: fmt.Sprintf("hits=%d", len(rs.Results))})
		}
	} else {
		results = append(results, CheckResult{Step: "rag_search", OK: false, Msg: fmt.Sprintf("status=%d", code)})
	}

	// RAG ask
	code, raw = requestJSON(handler, http.MethodPost, "/api/rag/ask", map[string]any{"query": "请总结学习笔记里今天的重点。", "top_k": 3})
	if code == http.StatusOK {
		var ans rag.AskResult
		if err := json.Unmarshal(raw, &ans); err == nil {
			results = append(results, CheckResult{Step: "rag_ask", OK: strings.TrimSpace(ans.Answer) != "", Msg: fmt.Sprintf("answer_len=%d", len(ans.Answer))})
		}
	} else {
		results = append(results, CheckResult{Step: "rag_ask", OK: false, Msg: fmt.Sprintf("status=%d", code)})
	}

	return results, nil
}

func requestJSON(h http.Handler, method, path string, body any) (int, []byte) {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func parseNoteResponse(raw []byte) (store.Note, string, error) {
	var n store.Note
	if err := json.Unmarshal(raw, &n); err == nil && n.ID > 0 {
		return n, "", nil
	}
	var wrap struct {
		Note         store.Note `json:"note"`
		IndexWarning string     `json:"index_warning"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return store.Note{}, "", fmt.Errorf("parse response: %w raw=%s", err, string(raw))
	}
	return wrap.Note, wrap.IndexWarning, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func sqliteFileFromDSN(dsn string) string {
	raw := strings.TrimSpace(dsn)
	if raw == "" {
		return ""
	}
	raw = strings.TrimPrefix(raw, "file:")
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}
