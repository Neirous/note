package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"note/internal/rag"
	"note/internal/store"
)

type fakeRagLLM struct{}

func (fakeRagLLM) Embed(_ context.Context, text string) ([]float64, error) {
	return []float64{float64(len(text)), 1}, nil
}

func (fakeRagLLM) Generate(_ context.Context, _ string) (string, error) {
	return "answer", nil
}

type spyGenerator struct {
	lastPrompt string
	response   string
	calls      int
}

func (g *spyGenerator) Generate(_ context.Context, prompt string) (string, error) {
	g.lastPrompt = prompt
	g.calls++
	if g.response != "" {
		return g.response, nil
	}
	return "# optimized", nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	return newTestServerWithGenerator(t, fakeRagLLM{})
}

func newTestServerWithGenerator(t *testing.T, generator rag.Generator) *Server {
	t.Helper()

	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	s := store.New(db)
	if err := s.InitSchema(context.Background()); err != nil {
		t.Fatal(err)
	}

	r := rag.NewService(s, fakeRagLLM{}, generator, rag.Config{MaxChunkChars: 400, TopK: 3})
	return NewServer(s, r)
}

func TestServerCreateGetAndFilterNote(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "t1",
		"markdown": "# hello",
		"tags":     []string{"project", "go"},
	}
	raw, _ := json.Marshal(createBody)

	req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var note store.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	if note.ID == 0 {
		t.Fatal("expected note id")
	}
	if len(note.Tags) == 0 {
		t.Fatal("expected tags")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/notes/1", nil)
	getRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/notes?tag=project", nil)
	listRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var notes []store.Note
	if err := json.Unmarshal(listRec.Body.Bytes(), &notes); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
}

func TestServerPinAndArchiveFlow(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "flow",
		"markdown": "content",
	}
	raw, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}

	var created store.Note
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	pinBody, _ := json.Marshal(map[string]bool{"value": true})
	pinReq := httptest.NewRequest(http.MethodPatch, "/api/notes/"+itoa(created.ID)+"/pin", bytes.NewReader(pinBody))
	pinReq.Header.Set("Content-Type", "application/json")
	pinRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(pinRec, pinReq)
	if pinRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on pin, got %d body=%s", pinRec.Code, pinRec.Body.String())
	}

	archiveBody, _ := json.Marshal(map[string]bool{"value": true})
	archiveReq := httptest.NewRequest(http.MethodPatch, "/api/notes/"+itoa(created.ID)+"/archive", bytes.NewReader(archiveBody))
	archiveReq.Header.Set("Content-Type", "application/json")
	archiveRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(archiveRec, archiveReq)
	if archiveRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on archive, got %d body=%s", archiveRec.Code, archiveRec.Body.String())
	}

	var archived store.Note
	if err := json.Unmarshal(archiveRec.Body.Bytes(), &archived); err != nil {
		t.Fatal(err)
	}
	if !archived.IsArchived {
		t.Fatal("expected archived note")
	}

	// Active list should exclude archived items by default.
	listReq := httptest.NewRequest(http.MethodGet, "/api/notes", nil)
	listRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}
	var active []store.Note
	if err := json.Unmarshal(listRec.Body.Bytes(), &active); err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active notes, got %d", len(active))
	}

	archivedReq := httptest.NewRequest(http.MethodGet, "/api/notes?archived=1", nil)
	archivedRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(archivedRec, archivedReq)
	if archivedRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", archivedRec.Code)
	}
	var archivedList []store.Note
	if err := json.Unmarshal(archivedRec.Body.Bytes(), &archivedList); err != nil {
		t.Fatal(err)
	}
	if len(archivedList) != 1 {
		t.Fatalf("expected 1 archived note, got %d", len(archivedList))
	}
}

func TestServerDeleteTag(t *testing.T) {
	srv := newTestServer(t)

	create := func(title string, tags []string) {
		body := map[string]any{
			"title":    title,
			"markdown": "content",
			"tags":     tags,
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	create("A", []string{"go", "api"})
	create("B", []string{"go"})

	delReq := httptest.NewRequest(http.MethodDelete, "/api/tags?tag=go", nil)
	delRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete tag, got %d body=%s", delRec.Code, delRec.Body.String())
	}

	listTagsReq := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	listTagsRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(listTagsRec, listTagsReq)
	if listTagsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on list tags, got %d body=%s", listTagsRec.Code, listTagsRec.Body.String())
	}

	var tags []string
	if err := json.Unmarshal(listTagsRec.Body.Bytes(), &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "api" {
		t.Fatalf("expected [api], got %+v", tags)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestServerBlocksSyncFlow(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "blocks",
		"markdown": "legacy",
	}
	raw, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}
	var created store.Note
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	replaceBody := map[string]any{
		"blocks": []map[string]any{
			{"type": "heading1", "content": "My Title", "level": 0},
			{"type": "todo", "content": "Ship", "checked": true, "level": 1},
		},
	}
	rawReplace, _ := json.Marshal(replaceBody)
	replaceReq := httptest.NewRequest(http.MethodPut, "/api/notes/"+itoa(created.ID)+"/blocks", bytes.NewReader(rawReplace))
	replaceReq.Header.Set("Content-Type", "application/json")
	replaceRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on replace blocks, got %d body=%s", replaceRec.Code, replaceRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(created.ID)+"/blocks", nil)
	getRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get blocks, got %d body=%s", getRec.Code, getRec.Body.String())
	}
	var blocks []store.NoteBlock
	if err := json.Unmarshal(getRec.Body.Bytes(), &blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[1].Level != 1 {
		t.Fatalf("expected second block level=1, got %d", blocks[1].Level)
	}

	noteReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(created.ID), nil)
	noteRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(noteRec, noteReq)
	if noteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on get note, got %d", noteRec.Code)
	}
	var updated store.Note
	if err := json.Unmarshal(noteRec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Markdown == "" {
		t.Fatal("expected markdown synced from blocks")
	}
	if updated.Markdown[:1] != "#" {
		t.Fatalf("unexpected markdown: %s", updated.Markdown)
	}
}

func TestServerBlocksLevelNormalization(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "levels",
		"markdown": "x",
	}
	raw, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}
	var created store.Note
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	replaceBody := map[string]any{
		"blocks": []map[string]any{
			{"type": "todo", "content": "a", "level": 4},
			{"type": "todo", "content": "b", "level": 5},
		},
	}
	rawReplace, _ := json.Marshal(replaceBody)
	replaceReq := httptest.NewRequest(http.MethodPut, "/api/notes/"+itoa(created.ID)+"/blocks", bytes.NewReader(rawReplace))
	replaceReq.Header.Set("Content-Type", "application/json")
	replaceRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", replaceRec.Code, replaceRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(created.ID)+"/blocks", nil)
	getRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
	var blocks []store.NoteBlock
	if err := json.Unmarshal(getRec.Body.Bytes(), &blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Level != 0 || blocks[1].Level != 1 {
		t.Fatalf("expected normalized levels [0,1], got [%d,%d]", blocks[0].Level, blocks[1].Level)
	}
}

func TestServerDuplicateAndExport(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "export me",
		"markdown": "# hello",
		"tags":     []string{"x"},
	}
	raw, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}
	var created store.Note
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	replaceBody := map[string]any{
		"blocks": []map[string]any{
			{"type": "heading1", "content": "Title", "level": 0},
			{"type": "todo", "content": "Task", "checked": true, "level": 1},
		},
	}
	rawReplace, _ := json.Marshal(replaceBody)
	replaceReq := httptest.NewRequest(http.MethodPut, "/api/notes/"+itoa(created.ID)+"/blocks", bytes.NewReader(rawReplace))
	replaceReq.Header.Set("Content-Type", "application/json")
	replaceRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 replace blocks, got %d body=%s", replaceRec.Code, replaceRec.Body.String())
	}

	dupReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+itoa(created.ID)+"/duplicate", nil)
	dupRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(dupRec, dupReq)
	if dupRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 duplicate, got %d body=%s", dupRec.Code, dupRec.Body.String())
	}
	var dup store.Note
	if err := json.Unmarshal(dupRec.Body.Bytes(), &dup); err != nil {
		t.Fatal(err)
	}
	if dup.ID == created.ID {
		t.Fatal("duplicate should have different id")
	}
	if dup.Title == created.Title {
		t.Fatal("duplicate title should be prefixed")
	}

	dupBlocksReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(dup.ID)+"/blocks", nil)
	dupBlocksRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(dupBlocksRec, dupBlocksReq)
	if dupBlocksRec.Code != http.StatusOK {
		t.Fatalf("expected 200 duplicate blocks, got %d body=%s", dupBlocksRec.Code, dupBlocksRec.Body.String())
	}
	var dupBlocks []store.NoteBlock
	if err := json.Unmarshal(dupBlocksRec.Body.Bytes(), &dupBlocks); err != nil {
		t.Fatal(err)
	}
	if len(dupBlocks) != 2 {
		t.Fatalf("expected 2 duplicate blocks, got %d", len(dupBlocks))
	}
	if dupBlocks[1].Type != "todo" || !dupBlocks[1].Checked || dupBlocks[1].Level != 1 {
		t.Fatalf("unexpected duplicate second block: %+v", dupBlocks[1])
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(created.ID)+"/export.md", nil)
	exportRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected 200 export, got %d", exportRec.Code)
	}
	if got := exportRec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/markdown") {
		t.Fatalf("unexpected content-type: %s", got)
	}
	if got := exportRec.Header().Get("Content-Disposition"); !strings.Contains(got, ".md") {
		t.Fatalf("unexpected content-disposition: %s", got)
	}
	body := exportRec.Body.String()
	if !strings.Contains(body, "# Title") || !strings.Contains(body, "- [x] Task") {
		t.Fatalf("unexpected export body: %q", body)
	}
}

func TestServerListNotesLite(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "lite",
		"markdown": "# body",
	}
	raw, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/notes?lite=1", nil)
	listRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRec.Code, listRec.Body.String())
	}
	var listed []store.Note
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 note, got %d", len(listed))
	}
	if listed[0].Markdown != "" || listed[0].HTML != "" {
		t.Fatalf("expected lite payload without content, got markdown=%q html=%q", listed[0].Markdown, listed[0].HTML)
	}
}

func TestServerRejectsNonFolderParent(t *testing.T) {
	srv := newTestServer(t)

	create := func(title string, tags []string) store.Note {
		t.Helper()
		body := map[string]any{
			"title":    title,
			"markdown": "content",
			"tags":     tags,
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		var note store.Note
		if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
			t.Fatal(err)
		}
		return note
	}

	fileParent := create("plain file", nil)
	folderParent := create("folder", []string{"folder"})

	childBody := map[string]any{
		"title":     "child",
		"markdown":  "content",
		"parent_id": fileParent.ID,
	}
	rawChild, _ := json.Marshal(childBody)
	childReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(rawChild))
	childReq.Header.Set("Content-Type", "application/json")
	childRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(childRec, childReq)
	if childRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-folder parent, got %d body=%s", childRec.Code, childRec.Body.String())
	}

	okBody := map[string]any{
		"title":     "allowed child",
		"markdown":  "content",
		"parent_id": folderParent.ID,
	}
	rawOK, _ := json.Marshal(okBody)
	okReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(rawOK))
	okReq.Header.Set("Content-Type", "application/json")
	okRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for folder parent, got %d body=%s", okRec.Code, okRec.Body.String())
	}
}

func TestServerResolvesPathLinksForHTMLButKeepsRawMarkdown(t *testing.T) {
	srv := newTestServer(t)

	create := func(title string, parentID *int64, tags []string, markdown string) store.Note {
		t.Helper()
		body := map[string]any{
			"title":    title,
			"markdown": markdown,
			"tags":     tags,
		}
		if parentID != nil {
			body["parent_id"] = *parentID
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		var note store.Note
		if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
			t.Fatal(err)
		}
		return note
	}

	folder := create("Backend", nil, []string{"folder"}, "# folder")
	target := create("API Guide", &folder.ID, nil, "# api")
	source := create("Index", nil, nil, "[Open API](Backend/API Guide)")

	getReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(source.ID), nil)
	getRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", getRec.Code, getRec.Body.String())
	}

	var fetched store.Note
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.Markdown != "[Open API](Backend/API Guide)" {
		t.Fatalf("expected raw markdown path to be preserved, got %q", fetched.Markdown)
	}
	if !strings.Contains(fetched.HTML, "note://"+itoa(target.ID)) {
		t.Fatalf("expected rendered HTML to contain note id link, got %q", fetched.HTML)
	}
}

func TestServerRAGAskAcceptsAnchorNoteID(t *testing.T) {
	srv := newTestServer(t)

	createBody := map[string]any{
		"title":    "后端工程",
		"markdown": "HTTP API 和错误处理",
		"tags":     []string{"backend"},
	}
	rawCreate, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(rawCreate))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	var created store.Note
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rawAsk, _ := json.Marshal(map[string]any{
		"query":   "总结后端",
		"top_k":   3,
		"note_id": created.ID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rag/ask", bytes.NewReader(rawAsk))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServerRAGAskAssistantModeUsesWorkbenchPrompt(t *testing.T) {
	gen := &spyGenerator{response: "按优先级执行计划"}
	srv := newTestServerWithGenerator(t, gen)

	createBody := map[string]any{
		"title":    "RAG 系统设计",
		"markdown": "检索增强生成用于从笔记库找上下文。",
		"tags":     []string{"rag"},
	}
	rawCreate, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(rawCreate))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	rawAsk, _ := json.Marshal(map[string]any{
		"query": "请优先参考以下用户添加的上下文回答。\n\n【助手工作台：计划任务】\n未完成计划数量：1\n- 完成论文终稿；日期：2026-05-22；时间：09:00 - 10:00；优先级：高\n\n用户问题：请整理未完成的所有计划。",
		"top_k": 7,
		"mode":  "assistant",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rag/ask", bytes.NewReader(rawAsk))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var out rag.AskResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Answer != "按优先级执行计划" {
		t.Fatalf("expected assistant answer, got %q", out.Answer)
	}
	if !strings.Contains(gen.lastPrompt, "独立的工作台 AI 助手") {
		t.Fatalf("expected assistant prompt identity, got %q", gen.lastPrompt)
	}
	if !strings.Contains(gen.lastPrompt, "不要因为检索上下文不足就直接回答") {
		t.Fatalf("expected prompt to avoid blank RAG answer, got %q", gen.lastPrompt)
	}
	if !strings.Contains(gen.lastPrompt, "完成论文终稿") {
		t.Fatalf("expected prompt to keep workbench context, got %q", gen.lastPrompt)
	}
}

func TestServerRAGAskLibraryModeUsesStrictLibraryPrompt(t *testing.T) {
	gen := &spyGenerator{response: "RAG 依赖检索到的笔记片段回答"}
	srv := newTestServerWithGenerator(t, gen)

	createBody := map[string]any{
		"title":    "RAG 系统设计",
		"markdown": "检索增强生成会先从全库笔记中召回相关片段，再基于上下文回答。",
		"tags":     []string{"rag"},
	}
	rawCreate, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(rawCreate))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	rawAsk, _ := json.Marshal(map[string]any{
		"query": "RAG 问答是基于什么回答的？",
		"top_k": 7,
		"mode":  "library",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/rag/ask", bytes.NewReader(rawAsk))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var out rag.AskResult
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Answer != "RAG 依赖检索到的笔记片段回答" {
		t.Fatalf("expected library answer, got %q", out.Answer)
	}
	if !strings.Contains(gen.lastPrompt, "全库问答助手") {
		t.Fatalf("expected library prompt identity, got %q", gen.lastPrompt)
	}
	if !strings.Contains(gen.lastPrompt, "不要凭空补充笔记中没有的事实") {
		t.Fatalf("expected strict grounding instruction, got %q", gen.lastPrompt)
	}
	if strings.Contains(gen.lastPrompt, "助手工作台：计划任务") {
		t.Fatalf("library prompt should not include workbench plan context, got %q", gen.lastPrompt)
	}
	if len(out.Contexts) == 0 {
		t.Fatalf("expected retrieved contexts")
	}
}

func TestServerOptimizeNoteUsesLinkedContext(t *testing.T) {
	gen := &spyGenerator{
		response: "```markdown\n# 优化后的笔记\n\n- 重点更清晰\n- 继续查看 [接口说明](Backend/API Guide)\n```",
	}
	srv := newTestServerWithGenerator(t, gen)

	create := func(title string, parentID *int64, tags []string, markdown string) store.Note {
		t.Helper()
		body := map[string]any{
			"title":    title,
			"markdown": markdown,
			"tags":     tags,
		}
		if parentID != nil {
			body["parent_id"] = *parentID
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/notes", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}
		var note store.Note
		if err := json.Unmarshal(rec.Body.Bytes(), &note); err != nil {
			t.Fatal(err)
		}
		return note
	}

	folder := create("Backend", nil, []string{"folder"}, "# backend")
	linked := create("API Guide", &folder.ID, nil, "接口定义\n\n- GET /api")
	source := create("Draft", nil, nil, "原始内容\n\n[接口说明](Backend/API Guide)")

	reqBody := map[string]any{
		"title":    "Draft",
		"markdown": "原始内容\n\n[接口说明](Backend/API Guide)",
	}
	raw, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/notes/"+itoa(source.ID)+"/optimize", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Markdown   string              `json:"markdown"`
		HTML       string              `json:"html"`
		References []optimizeReference `json:"references"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Markdown, "# 优化后的笔记") {
		t.Fatalf("expected optimized markdown, got %q", resp.Markdown)
	}
	if len(resp.References) != 1 || resp.References[0].NoteID != linked.ID {
		t.Fatalf("expected linked note reference, got %+v", resp.References)
	}
	if !strings.Contains(resp.HTML, "<h1>优化后的笔记</h1>") {
		t.Fatalf("expected rendered html, got %q", resp.HTML)
	}
	if !strings.Contains(gen.lastPrompt, "原始内容") {
		t.Fatalf("expected prompt to include source markdown, got %q", gen.lastPrompt)
	}
	if !strings.Contains(gen.lastPrompt, linked.Markdown) {
		t.Fatalf("expected prompt to include linked markdown, got %q", gen.lastPrompt)
	}
}
