package rag

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"note/internal/store"
)

type fakeLLM struct{}

func (fakeLLM) Embed(_ context.Context, text string) ([]float64, error) {
	text = strings.ToLower(text)
	scoreGo := float64(strings.Count(text, "go"))
	scoreDB := float64(strings.Count(text, "db"))
	return []float64{scoreGo, scoreDB}, nil
}

func (fakeLLM) Generate(_ context.Context, _ string) (string, error) {
	return "ok", nil
}

type topicEmbedder struct{}

func (topicEmbedder) Embed(_ context.Context, text string) ([]float64, error) {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "当前笔记"):
		return []float64{1, 0}, nil
	case strings.Contains(text, "stride") || strings.Contains(text, "威胁"):
		return []float64{1, 0}, nil
	case strings.Contains(text, "slo") || strings.Contains(text, "可观测"):
		return []float64{0, 1}, nil
	default:
		return []float64{0, 0}, nil
	}
}

type promptSpyGenerator struct {
	prompt string
}

func (g *promptSpyGenerator) Generate(_ context.Context, prompt string) (string, error) {
	g.prompt = prompt
	return "ok", nil
}

func TestServiceSearch(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := store.New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n1, err := s.CreateNote(ctx, store.NoteInput{Title: "Go", Markdown: "go go go", HTML: "<p>go go go</p>"})
	if err != nil {
		t.Fatal(err)
	}
	n2, err := s.CreateNote(ctx, store.NoteInput{Title: "DB", Markdown: "db db", HTML: "<p>db db</p>"})
	if err != nil {
		t.Fatal(err)
	}

	r := NewService(s, fakeLLM{}, fakeLLM{}, Config{MaxChunkChars: 200, TopK: 3})
	if err := r.IndexNote(ctx, n1.ID, n1.Markdown); err != nil {
		t.Fatal(err)
	}
	if err := r.IndexNote(ctx, n2.ID, n2.Markdown); err != nil {
		t.Fatal(err)
	}

	res, err := r.Search(ctx, "go", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected results")
	}
	if res.Results[0].NoteID != n1.ID {
		t.Fatalf("expected first result note %d, got %d", n1.ID, res.Results[0].NoteID)
	}
}

func TestAskWithAnchorUsesCurrentNoteWhenIndexIsMissing(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := store.New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	current, err := s.CreateNote(ctx, store.NoteInput{
		Title:    "可观测性与 SLO 设计",
		Markdown: "日志、指标和追踪用于可观测性材料。SLO 需要围绕用户旅程、错误预算和告警疲劳来设计。",
		HTML:     "<p>SLO</p>",
		Tags:     []string{"observability", "slo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateNote(ctx, store.NoteInput{
		Title:    "安全威胁建模入门",
		Markdown: "威胁建模使用 STRIDE、边界分析和风险矩阵来识别安全问题。",
		HTML:     "<p>STRIDE</p>",
		Tags:     []string{"security"},
	})
	if err != nil {
		t.Fatal(err)
	}

	gen := &promptSpyGenerator{}
	r := NewService(s, topicEmbedder{}, gen, Config{MaxChunkChars: 200, TopK: 5})
	if err := r.IndexNote(ctx, other.ID, other.Markdown); err != nil {
		t.Fatal(err)
	}

	res, err := r.AskWithOptions(ctx, "请问当前笔记是什么", 5, SearchOptions{AnchorNoteID: current.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contexts) == 0 {
		t.Fatal("expected contexts")
	}
	if res.Contexts[0].NoteID != current.ID {
		t.Fatalf("expected current note first, got note %d (%q)", res.Contexts[0].NoteID, res.Contexts[0].NoteTitle)
	}
	if !strings.Contains(gen.prompt, "CURRENT_NOTE") || !strings.Contains(gen.prompt, "可观测性与 SLO 设计") {
		t.Fatalf("expected prompt to mark current note with title, got:\n%s", gen.prompt)
	}
	currentAt := strings.Index(gen.prompt, "CURRENT_NOTE")
	relatedAt := strings.Index(gen.prompt, "安全威胁建模入门")
	if currentAt < 0 || relatedAt < 0 || currentAt > relatedAt {
		t.Fatalf("expected current note context before related security note, got:\n%s", gen.prompt)
	}
}

func TestServiceSearchWithAnchorBoost(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := store.New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	root, err := s.CreateNote(ctx, store.NoteInput{Title: "后端工程", Markdown: "HTTP API 设计", HTML: "<p>HTTP API 设计</p>", Tags: []string{"backend"}})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateNote(ctx, store.NoteInput{ParentID: &root.ID, Title: "错误处理", Markdown: "状态码和 handler 约定", HTML: "<p>状态码和 handler 约定</p>", Tags: []string{"backend"}})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateNote(ctx, store.NoteInput{Title: "旅行计划", Markdown: "后端街区的餐厅推荐", HTML: "<p>后端街区的餐厅推荐</p>", Tags: []string{"travel"}})
	if err != nil {
		t.Fatal(err)
	}

	r := NewService(s, fakeLLM{}, fakeLLM{}, Config{MaxChunkChars: 200, TopK: 3})
	for _, note := range []store.Note{root, child, other} {
		if err := r.IndexNote(ctx, note.ID, note.Markdown); err != nil {
			t.Fatal(err)
		}
	}

	res, err := r.SearchWithOptions(ctx, "总结后端", 3, SearchOptions{AnchorNoteID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 {
		t.Fatal("expected results")
	}
	if res.Results[0].NoteID != root.ID && res.Results[0].NoteID != child.ID {
		t.Fatalf("expected anchor-related note first, got %d", res.Results[0].NoteID)
	}
}
