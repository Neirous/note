package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestStoreCRUDAndFilter(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := New(db)
	ctx := context.Background()

	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n, err := s.CreateNote(ctx, NoteInput{
		Title:    "Go Tips",
		Markdown: "# hello",
		HTML:     "<h1>hello</h1>",
		Tags:     []string{"go", "backend"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n.ID == 0 {
		t.Fatal("expected id")
	}
	if n.Status != "unfinished" {
		t.Fatalf("expected default unfinished status, got %s", n.Status)
	}
	if len(n.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(n.Tags))
	}

	n2, err := s.UpdateNote(ctx, n.ID, NoteInput{
		Title:    "Database Tips",
		Markdown: "body",
		HTML:     "<p>body</p>",
		Tags:     []string{"db"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n2.Title != "Database Tips" {
		t.Fatalf("expected updated title, got %s", n2.Title)
	}
	if len(n2.Tags) != 1 || n2.Tags[0] != "db" {
		t.Fatalf("expected tags=[db], got %+v", n2.Tags)
	}

	list, err := s.ListNotes(ctx, NoteFilter{Query: "Database"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 note, got %d", len(list))
	}

	list, err = s.ListNotes(ctx, NoteFilter{Tag: "db"})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 note by tag, got %d", len(list))
	}

	tags, err := s.ListDistinctTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "db" {
		t.Fatalf("unexpected tags: %+v", tags)
	}

	if err := s.DeleteNote(ctx, n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetNote(ctx, n.ID); err != ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestStoreNoteStatusAndKnowledgeCards(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n, err := s.CreateNote(ctx, NoteInput{Title: "Status", Markdown: "body", HTML: "<p>body</p>"})
	if err != nil {
		t.Fatal(err)
	}
	n, err = s.SetNoteStatus(ctx, n.ID, "completed")
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != "completed" {
		t.Fatalf("expected completed, got %s", n.Status)
	}

	card, err := s.CreateKnowledgeCard(ctx, KnowledgeCardInput{
		Front: "什么是 RAG？",
		Back:  "检索增强生成。",
		Tags:  []string{"AI", "rag"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if card.ID == 0 || card.Status != "active" || card.ReviewStage != 0 {
		t.Fatalf("unexpected card: %+v", card)
	}
	if len(card.Tags) != 2 || card.Tags[0] != "ai" || card.Tags[1] != "rag" {
		t.Fatalf("unexpected tags: %+v", card.Tags)
	}

	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	card, err = s.ReviewKnowledgeCard(ctx, card.ID, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if card.ReviewStage != 1 || card.Status != "active" || card.NextReviewAt == nil {
		t.Fatalf("expected stage 1 active card, got %+v", card)
	}
	if got := card.NextReviewAt.Sub(now); got != 24*time.Hour {
		t.Fatalf("expected next review in 1 day, got %s", got)
	}

	card, err = s.ReviewKnowledgeCard(ctx, card.ID, false, now)
	if err != nil {
		t.Fatal(err)
	}
	if card.ReviewStage != 1 || card.Status != "active" || card.NextReviewAt == nil {
		t.Fatalf("expected forgotten card to stay in current stage, got %+v", card)
	}
	if got := card.NextReviewAt.Sub(now); got != 0 {
		t.Fatalf("expected forgotten card to stay due now, got %s", got)
	}

	for i := 0; i < 6; i++ {
		card, err = s.ReviewKnowledgeCard(ctx, card.ID, true, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if card.Status != "mastered" || card.ReviewStage != 6 {
		t.Fatalf("expected mastered stage 6, got %+v", card)
	}

	cards, err := s.ListKnowledgeCards(ctx, CardFilter{IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if err := s.DeleteKnowledgeCard(ctx, card.ID); err != nil {
		t.Fatal(err)
	}
}

func TestStoreResearchSessions(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := s.CreateResearchSession(ctx, "Go 后端设计", []byte(`{"topic":"Go 后端设计","summary":"summary"}`))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Topic != "Go 后端设计" {
		t.Fatalf("unexpected session: %+v", created)
	}

	items, err := s.ListResearchSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("expected saved session, got %+v", items)
	}

	updated, err := s.CreateResearchSession(ctx, "go 后端设计", []byte(`{"topic":"go 后端设计","summary":"updated"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected same-topic session to update id=%d, got %+v", created.ID, updated)
	}
	items, err = s.ListResearchSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Result, "updated") {
		t.Fatalf("expected updated single session, got %+v", items)
	}

	if err := s.DeleteResearchSession(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListResearchSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no sessions after delete, got %+v", items)
	}
}

func TestStoreRecommendationSessions(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := s.CreateRecommendationSession(ctx, "Go 学习资源", []byte(`{"topic":"Go 学习资源","summary":"summary"}`))
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Topic != "Go 学习资源" {
		t.Fatalf("unexpected session: %+v", created)
	}

	items, err := s.ListRecommendationSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("expected saved session, got %+v", items)
	}

	updated, err := s.CreateRecommendationSession(ctx, "go 学习资源", []byte(`{"topic":"go 学习资源","summary":"updated"}`))
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected same-topic session to update id=%d, got %+v", created.ID, updated)
	}
	items, err = s.ListRecommendationSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Result, "updated") {
		t.Fatalf("expected updated single session, got %+v", items)
	}

	if err := s.DeleteRecommendationSession(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListRecommendationSessions(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no sessions after delete, got %+v", items)
	}
}

func TestStoreParentHierarchy(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	parent, err := s.CreateNote(ctx, NoteInput{Title: "Parent", Markdown: "p", HTML: "<p>p</p>"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateNote(ctx, NoteInput{ParentID: &parent.ID, Title: "Child", Markdown: "c", HTML: "<p>c</p>"})
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("expected parent id=%d, got %+v", parent.ID, child.ParentID)
	}
}

func TestStorePinAndArchive(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n, err := s.CreateNote(ctx, NoteInput{Title: "N1", Markdown: "body", HTML: "<p>body</p>"})
	if err != nil {
		t.Fatal(err)
	}

	n, err = s.SetPinned(ctx, n.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !n.IsPinned {
		t.Fatal("expected pinned=true")
	}

	n, err = s.SetArchived(ctx, n.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !n.IsArchived {
		t.Fatal("expected archived=true")
	}
	if n.IsPinned {
		t.Fatal("expected pinned=false after archiving")
	}

	if _, err := s.SetPinned(ctx, n.ID, true); err != ErrInvalidState {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}

	active, err := s.ListNotes(ctx, NoteFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active notes, got %d", len(active))
	}

	archived, err := s.ListNotes(ctx, NoteFilter{OnlyArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected one archived note, got %d", len(archived))
	}
}

func TestStoreBlocksReplaceAndList(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n, err := s.CreateNote(ctx, NoteInput{Title: "B", Markdown: "", HTML: ""})
	if err != nil {
		t.Fatal(err)
	}
	err = s.ReplaceNoteBlocks(ctx, n.ID, []NoteBlockInput{
		{Type: "heading1", Content: "Title", Level: 0},
		{Type: "todo", Content: "Task", Checked: true, Level: 2},
		{Type: "code", Content: "fmt.Println(1)"},
	})
	if err != nil {
		t.Fatal(err)
	}

	blocks, err := s.ListNoteBlocks(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	if blocks[1].Type != "todo" || !blocks[1].Checked {
		t.Fatalf("unexpected second block: %+v", blocks[1])
	}
	if blocks[1].Level != 2 {
		t.Fatalf("expected level=2, got %d", blocks[1].Level)
	}
}

func TestStoreReviewQuestionsCRUD(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n, err := s.CreateNote(ctx, NoteInput{Title: "Review", Markdown: "# Review", HTML: "<h1>Review</h1>"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.CreateReviewQuestion(ctx, n.ID, ReviewQuestionInput{
		Question: "核心概念是什么？",
		Answer:   "用自己的话解释。",
		Source:   "ai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if q.ID == 0 || q.Source != "ai" {
		t.Fatalf("unexpected question: %+v", q)
	}

	updated, err := s.UpdateReviewQuestion(ctx, n.ID, q.ID, ReviewQuestionInput{
		Question: "核心概念如何落地？",
		Answer:   "结合例子回答。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Question != "核心概念如何落地？" || updated.Source != "ai" {
		t.Fatalf("unexpected updated question: %+v", updated)
	}

	items, err := s.ListReviewQuestions(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 question, got %d", len(items))
	}
	if err := s.DeleteReviewQuestion(ctx, n.ID, q.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListReviewQuestions(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no questions after delete, got %+v", items)
	}
}

func TestStoreListNotesLite(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = s.CreateNote(ctx, NoteInput{
		Title:    "Lite",
		Markdown: "# heavy",
		HTML:     "<h1>heavy</h1>",
		Tags:     []string{"x"},
	})
	if err != nil {
		t.Fatal(err)
	}

	list, err := s.ListNotesLite(ctx, NoteFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 note, got %d", len(list))
	}
	if list[0].Markdown != "" || list[0].HTML != "" {
		t.Fatalf("expected lite note without content, got markdown=%q html=%q", list[0].Markdown, list[0].HTML)
	}
}

func TestStoreDeleteTag(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db)
	ctx := context.Background()
	if err := s.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}

	n1, err := s.CreateNote(ctx, NoteInput{Title: "N1", Markdown: "a", HTML: "<p>a</p>", Tags: []string{"go", "api"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateNote(ctx, NoteInput{Title: "N2", Markdown: "b", HTML: "<p>b</p>", Tags: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteTag(ctx, "go"); err != nil {
		t.Fatal(err)
	}

	tags, err := s.ListDistinctTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "api" {
		t.Fatalf("expected [api], got %+v", tags)
	}

	reloaded, err := s.GetNote(ctx, n1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Tags) != 1 || reloaded.Tags[0] != "api" {
		t.Fatalf("expected note tags [api], got %+v", reloaded.Tags)
	}
}
