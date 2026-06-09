package main

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

type checkResult struct {
	step string
	ok   bool
	msg  string
}

func main() {
	results := make([]checkResult, 0, 16)

	dsn := getenv("E2E_DSN", "file:e2e-check.db?_pragma=busy_timeout(5000)")
	dbFile := sqliteFileFromDSN(dsn)
	cleanup := getenv("E2E_CLEANUP", "1") == "1"
	if cleanup && dbFile != "" {
		_ = os.Remove(dbFile)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()
	if cleanup && dbFile != "" {
		defer os.Remove(dbFile)
	}

	st := store.New(db)
	if err := st.InitSchema(context.Background()); err != nil {
		fatalf("init schema: %v", err)
	}

	baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	apiKey := os.Getenv("OPENAI_API_KEY")
	embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
	chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
	client := llm.NewOpenAICompatibleClient(&http.Client{Timeout: 90 * time.Second}, baseURL, apiKey, embedModel, chatModel)
	ragService := rag.NewService(st, client, client, rag.Config{
		MaxChunkChars: 800,
		TopK:          5,
	})
	srv := api.NewServer(st, ragService)
	handler := srv.Routes()

	noteBodies := []map[string]any{
		{
			"title": "旅行计划 2026",
			"markdown": strings.TrimSpace(`
# 旅行计划 2026

> 目标：春季完成一次 5 天城市漫游

## TODO
- [ ] 订机票
- [x] 预估预算

## 预算表
| 项目 | 金额 |
| --- | --- |
| 交通 | 3200 |
| 住宿 | 2600 |

~~~text
Pack: passport, camera, power bank
~~~
`),
			"tags": []string{"travel", "plan"},
		},
		{
			"title": "Go 学习笔记",
			"markdown": strings.TrimSpace(`
# Go 学习笔记

## Slice
- append 可能触发扩容
- 传参要关注底层数组共享

## 示例
~~~go
func sum(nums []int) int {
  s := 0
  for _, n := range nums {
    s += n
  }
  return s
}
~~~
`),
			"tags": []string{"go", "backend"},
		},
		{
			"title": "阅读清单",
			"markdown": strings.TrimSpace(`
# 阅读清单

1. 《Designing Data-Intensive Applications》
2. 《Clean Architecture》

> 每周至少读 2 章并输出摘要
`),
			"tags": []string{"reading"},
		},
	}

	var noteIDs []int64
	for i, body := range noteBodies {
		code, raw := requestJSON(handler, http.MethodPost, "/api/notes", body)
		if code != http.StatusCreated {
			results = append(results, checkResult{
				step: fmt.Sprintf("create_note_%d", i+1),
				ok:   false,
				msg:  fmt.Sprintf("status=%d body=%s", code, string(raw)),
			})
			continue
		}
		note, warn, parseErr := parseNoteResponse(raw)
		if parseErr != nil {
			results = append(results, checkResult{step: fmt.Sprintf("create_note_%d", i+1), ok: false, msg: parseErr.Error()})
			continue
		}
		noteIDs = append(noteIDs, note.ID)
		msg := fmt.Sprintf("id=%d title=%q", note.ID, note.Title)
		if warn != "" {
			msg += " index_warning=" + warn
		}
		results = append(results, checkResult{step: fmt.Sprintf("create_note_%d", i+1), ok: true, msg: msg})
	}

	code, raw := requestJSON(handler, http.MethodGet, "/api/notes", nil)
	if code == http.StatusOK {
		var listed []store.Note
		if err := json.Unmarshal(raw, &listed); err == nil {
			results = append(results, checkResult{step: "list_notes", ok: len(listed) >= 3, msg: fmt.Sprintf("count=%d", len(listed))})
		} else {
			results = append(results, checkResult{step: "list_notes", ok: false, msg: err.Error()})
		}
	} else {
		results = append(results, checkResult{step: "list_notes", ok: false, msg: fmt.Sprintf("status=%d body=%s", code, string(raw))})
	}

	code, raw = requestJSON(handler, http.MethodGet, "/api/notes?q=Go", nil)
	if code == http.StatusOK {
		var listed []store.Note
		if err := json.Unmarshal(raw, &listed); err == nil {
			ok := false
			for _, n := range listed {
				if strings.Contains(n.Title, "Go") {
					ok = true
					break
				}
			}
			results = append(results, checkResult{step: "query_notes", ok: ok, msg: fmt.Sprintf("matched=%d", len(listed))})
		} else {
			results = append(results, checkResult{step: "query_notes", ok: false, msg: err.Error()})
		}
	} else {
		results = append(results, checkResult{step: "query_notes", ok: false, msg: fmt.Sprintf("status=%d body=%s", code, string(raw))})
	}

	if len(noteIDs) >= 2 {
		updateBody := map[string]any{
			"title": "Go 学习笔记（已更新）",
			"markdown": strings.TrimSpace(`
# Go 学习笔记（已更新）

## 今日重点
- map 并发读写要加锁
- context 用于超时与取消

## 命令
~~~bash
go test ./...
~~~
`),
			"tags": []string{"go", "backend", "updated"},
		}
		path := fmt.Sprintf("/api/notes/%d", noteIDs[1])
		code, raw = requestJSON(handler, http.MethodPut, path, updateBody)
		if code == http.StatusOK {
			n, warn, parseErr := parseNoteResponse(raw)
			ok := parseErr == nil && strings.Contains(n.Title, "已更新")
			msg := fmt.Sprintf("title=%q", n.Title)
			if warn != "" {
				msg += " index_warning=" + warn
			}
			if parseErr != nil {
				msg = parseErr.Error()
			}
			results = append(results, checkResult{step: "update_note", ok: ok, msg: msg})
		} else {
			results = append(results, checkResult{step: "update_note", ok: false, msg: fmt.Sprintf("status=%d body=%s", code, string(raw))})
		}
	}

	if len(noteIDs) >= 3 {
		delPath := fmt.Sprintf("/api/notes/%d", noteIDs[2])
		code, raw = requestJSON(handler, http.MethodDelete, delPath, nil)
		results = append(results, checkResult{
			step: "delete_note",
			ok:   code == http.StatusOK,
			msg:  fmt.Sprintf("status=%d body=%s", code, strings.TrimSpace(string(raw))),
		})
	}

	code, raw = requestJSON(handler, http.MethodGet, "/api/notes", nil)
	if code == http.StatusOK {
		var listed []store.Note
		_ = json.Unmarshal(raw, &listed)
		results = append(results, checkResult{step: "list_after_delete", ok: len(listed) >= 2, msg: fmt.Sprintf("count=%d", len(listed))})
	}

	code, raw = requestJSON(handler, http.MethodPost, "/api/rag/search", map[string]any{
		"query": "Go 并发读写和 context 的重点",
		"top_k": 3,
	})
	if code == http.StatusOK {
		var rs rag.SearchResult
		if err := json.Unmarshal(raw, &rs); err == nil {
			results = append(results, checkResult{step: "rag_search", ok: len(rs.Results) > 0, msg: fmt.Sprintf("hits=%d", len(rs.Results))})
		} else {
			results = append(results, checkResult{step: "rag_search", ok: false, msg: err.Error()})
		}
	} else {
		results = append(results, checkResult{step: "rag_search", ok: false, msg: fmt.Sprintf("status=%d body=%s", code, string(raw))})
	}

	code, raw = requestJSON(handler, http.MethodPost, "/api/rag/ask", map[string]any{
		"query": "请总结学习笔记里今天的重点。",
		"top_k": 3,
	})
	if code == http.StatusOK {
		var ans rag.AskResult
		if err := json.Unmarshal(raw, &ans); err == nil {
			ok := strings.TrimSpace(ans.Answer) != ""
			msg := fmt.Sprintf("answer_len=%d contexts=%d", len(ans.Answer), len(ans.Contexts))
			results = append(results, checkResult{step: "rag_ask", ok: ok, msg: msg})
		} else {
			results = append(results, checkResult{step: "rag_ask", ok: false, msg: err.Error()})
		}
	} else {
		results = append(results, checkResult{step: "rag_ask", ok: false, msg: fmt.Sprintf("status=%d body=%s", code, string(raw))})
	}

	printResults(results)
	if hasFailure(results) {
		os.Exit(1)
	}
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
	if wrap.Note.ID == 0 {
		return store.Note{}, wrap.IndexWarning, fmt.Errorf("note missing in response: %s", string(raw))
	}
	return wrap.Note, wrap.IndexWarning, nil
}

func printResults(results []checkResult) {
	fmt.Println("E2E CHECK RESULTS")
	for _, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
		}
		fmt.Printf("- [%s] %s -> %s\n", status, r.step, r.msg)
	}
}

func hasFailure(results []checkResult) bool {
	for _, r := range results {
		if !r.ok {
			return true
		}
	}
	return false
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

func fatalf(format string, args ...any) {
	fmt.Printf("FATAL: "+format+"\n", args...)
	os.Exit(1)
}
