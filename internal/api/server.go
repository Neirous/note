package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yuin/goldmark"

	"note/internal/rag"
	"note/internal/store"
)

type Server struct {
	store *store.Store
	rag   *rag.Service
	md    goldmark.Markdown
}

func NewServer(store *store.Store, rag *rag.Service) *Server {
	return &Server{
		store: store,
		rag:   rag,
		md:    goldmark.New(),
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", s.handleHealthz)

	r.Route("/api", func(r chi.Router) {
		r.Get("/notes", s.handleListNotes)
		r.Get("/tags", s.handleListTags)
		r.Delete("/tags", s.handleDeleteTag)
		r.Post("/notes", s.handleCreateNote)
		r.Get("/notes/{id}", s.handleGetNote)
		r.Post("/notes/{id}/duplicate", s.handleDuplicateNote)
		r.Post("/notes/{id}/optimize", s.handleOptimizeNote)
		r.Get("/notes/{id}/recommendations", s.handleNoteRecommendations)
		r.Get("/notes/{id}/links", s.handleNoteLinks)
		r.Get("/notes/{id}/suggest-tags", s.handleSuggestTags)
		r.Get("/notes/{id}/insights", s.handleNoteInsights)
		r.Get("/notes/{id}/review-questions", s.handleListReviewQuestions)
		r.Post("/notes/{id}/review-questions", s.handleCreateReviewQuestion)
		r.Post("/notes/{id}/review-questions/generate", s.handleGenerateReviewQuestions)
		r.Put("/notes/{id}/review-questions/{questionID}", s.handleUpdateReviewQuestion)
		r.Delete("/notes/{id}/review-questions/{questionID}", s.handleDeleteReviewQuestion)
		r.Get("/notes/{id}/export.md", s.handleExportMarkdown)
		r.Get("/notes/{id}/blocks", s.handleListBlocks)
		r.Put("/notes/{id}/blocks", s.handleReplaceBlocks)
		r.Put("/notes/{id}", s.handleUpdateNote)
		r.Patch("/notes/{id}/status", s.handleSetNoteStatus)
		r.Patch("/notes/{id}/pin", s.handlePinNote)
		r.Patch("/notes/{id}/archive", s.handleArchiveNote)
		r.Delete("/notes/{id}", s.handleDeleteNote)
		r.Get("/cards/review/due", s.handleDueKnowledgeCards)
		r.Get("/cards", s.handleListKnowledgeCards)
		r.Post("/cards", s.handleCreateKnowledgeCard)
		r.Get("/cards/{id}", s.handleGetKnowledgeCard)
		r.Put("/cards/{id}", s.handleUpdateKnowledgeCard)
		r.Delete("/cards/{id}", s.handleDeleteKnowledgeCard)
		r.Post("/cards/{id}/review", s.handleReviewKnowledgeCard)
		r.Get("/workspace/dashboard", s.handleWorkspaceDashboard)
		r.Get("/workspace/graph", s.handleWorkspaceGraph)
		r.Post("/workspace/quality-evaluation", s.handleWorkspaceQualityEvaluation)
		r.Post("/research/session", s.handleResearchSession)
		r.Get("/research/sessions", s.handleListResearchSessions)
		r.Delete("/research/sessions/{id}", s.handleDeleteResearchSession)
		r.Post("/render", s.handleRenderMarkdown)
		r.Get("/review", s.handleDailyReview)
		r.Post("/recommend", s.handleAIRecommendation)
		r.Get("/recommend/sessions", s.handleListRecommendationSessions)
		r.Delete("/recommend/sessions/{id}", s.handleDeleteRecommendationSession)
		r.Post("/writing/weekly-report", s.handleCreateWeeklyReport)
		r.Get("/tasks", s.handleTasks)
		r.Get("/templates", s.handleTemplates)
		r.Post("/templates/{key}/notes", s.handleCreateFromTemplate)
		r.Post("/import", s.handleImportNote)

		r.Post("/rag/search", s.handleRAGSearch)
		r.Post("/rag/ask", s.handleRAGAsk)
	})

	fileServer(r, "/", http.Dir("web/static"))
	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type noteUpsertRequest struct {
	Title    string   `json:"title"`
	Markdown string   `json:"markdown"`
	ParentID *int64   `json:"parent_id"`
	Tags     []string `json:"tags"`
}

var wikiLinkRE = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
var markdownLinkRE = regexp.MustCompile(`(!?)\[([^\]]+)\]\(([^)]+)\)`)
var externalLinkRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z\d+.-]*:`)

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := store.NoteFilter{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Tag:             strings.TrimSpace(r.URL.Query().Get("tag")),
		IncludeArchived: parseBool(r.URL.Query().Get("include_archived")),
		OnlyArchived:    parseBool(r.URL.Query().Get("archived")),
	}
	lite := parseBool(r.URL.Query().Get("lite"))

	var notes []store.Note
	var err error
	if lite {
		notes, err = s.store.ListNotesLite(ctx, filter)
	} else {
		notes, err = s.store.ListNotes(ctx, filter)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if notes == nil {
		notes = []store.Note{}
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tags, err := s.store.ListDistinctTags(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		var req struct {
			Tag string `json:"tag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			tag = strings.TrimSpace(req.Tag)
		}
	}
	if tag == "" {
		writeErrMsg(w, http.StatusBadRequest, "tag is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.store.DeleteTag(ctx, tag); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"tag": tag,
	})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDuplicateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	src, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	dupTitle := "Copy of " + strings.TrimSpace(src.Title)
	if strings.TrimSpace(src.Title) == "" {
		dupTitle = "Copy of Untitled"
	}

	created, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: src.ParentID,
		Title:    dupTitle,
		Markdown: src.Markdown,
		HTML:     src.HTML,
		Tags:     src.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	srcBlocks, err := s.store.ListNoteBlocks(ctx, src.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(srcBlocks) > 0 {
		inputs := make([]store.NoteBlockInput, 0, len(srcBlocks))
		for _, b := range srcBlocks {
			inputs = append(inputs, store.NoteBlockInput{
				Type:    b.Type,
				Content: b.Content,
				Checked: b.Checked,
				Level:   b.Level,
			})
		}
		if err := s.store.ReplaceNoteBlocks(ctx, created.ID, inputs); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}

	if err := s.rag.IndexNote(ctx, created.ID, created.Markdown); err != nil {
		log.Printf("index note %d warning: %v", created.ID, err)
		writeJSON(w, http.StatusCreated, map[string]any{
			"note":          created,
			"index_warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleExportMarkdown(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	filename := sanitizeFilename(n.Title)
	if filename == "" {
		filename = "note"
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%d.md"`, filename, n.ID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(n.Markdown))
}

func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	blocks, err := s.store.ListNoteBlocks(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(blocks) == 0 && strings.TrimSpace(n.Markdown) != "" {
		blocks = parseMarkdownToBlocks(id, n.Markdown)
	}
	if blocks == nil {
		blocks = []store.NoteBlock{}
	}
	writeJSON(w, http.StatusOK, blocks)
}

func (s *Server) handleReplaceBlocks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Blocks []store.NoteBlockInput `json:"blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Blocks = normalizeBlockInputs(req.Blocks)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.store.ReplaceNoteBlocks(ctx, id, req.Blocks); errors.Is(err, store.ErrInvalidBlock) {
		writeErrMsg(w, http.StatusBadRequest, "invalid block type")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	blocks, err := s.store.ListNoteBlocks(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	markdown := blocksToMarkdown(blocks)
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	updated, err := s.store.UpdateNote(ctx, id, store.NoteInput{
		ParentID: n.ParentID,
		Title:    n.Title,
		Markdown: markdown,
		HTML:     html,
		Tags:     n.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, updated.ID, updated.Markdown); err != nil {
		log.Printf("index note %d warning: %v", updated.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{
			"note":          updated,
			"blocks":        blocks,
			"index_warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"note":   updated,
		"blocks": blocks,
	})
}

func (s *Server) handlePinNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Value bool `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetPinned(ctx, id, req.Value)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "cannot pin archived note")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleArchiveNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Value bool `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetArchived(ctx, id, req.Value)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req noteUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.validateParentFolder(ctx, req.ParentID); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusBadRequest, "parent note not found")
		return
	} else if err != nil {
		writeErrMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: req.ParentID,
		Title:    req.Title,
		Markdown: req.Markdown,
		HTML:     html,
		Tags:     req.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, n.ID, n.Markdown); err != nil {
		log.Printf("index note %d warning: %v", n.ID, err)
		writeJSON(w, http.StatusCreated, map[string]any{
			"note":          n,
			"index_warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req noteUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if req.ParentID != nil && *req.ParentID == id {
		writeErrMsg(w, http.StatusBadRequest, "parent_id cannot be self")
		return
	}
	if err := s.validateParentFolder(ctx, req.ParentID); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusBadRequest, "parent note not found")
		return
	} else if err != nil {
		writeErrMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.UpdateNote(ctx, id, store.NoteInput{
		ParentID: req.ParentID,
		Title:    req.Title,
		Markdown: req.Markdown,
		HTML:     html,
		Tags:     req.Tags,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, n.ID, n.Markdown); err != nil {
		log.Printf("index note %d warning: %v", n.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{
			"note":          n,
			"index_warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleSetNoteStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "unfinished" && status != "completed" {
		writeErrMsg(w, http.StatusBadRequest, "status must be unfinished or completed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetNoteStatus(ctx, id, status)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.store.DeleteNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRenderMarkdown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	html, err := s.renderMarkdown(req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": html})
}

func (s *Server) handleRAGSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query  string `json:"query"`
		TopK   int    `json:"top_k"`
		NoteID *int64 `json:"note_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
		AnchorNoteID: valueOrZero(req.NoteID),
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRAGAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query  string `json:"query"`
		TopK   int    `json:"top_k"`
		NoteID *int64 `json:"note_id"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if strings.EqualFold(req.Mode, "library") {
		searchResult, searchErr := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
			AnchorNoteID: valueOrZero(req.NoteID),
		})
		if searchErr != nil {
			writeErr(w, http.StatusBadGateway, searchErr)
			return
		}
		answer, err := s.rag.Generate(ctx, buildLibraryRAGPrompt(req.Query, searchResult.Results, valueOrZero(req.NoteID)))
		if err != nil {
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, rag.AskResult{
			Query:    req.Query,
			Answer:   answer,
			Contexts: searchResult.Results,
		})
		return
	}
	if strings.EqualFold(req.Mode, "assistant") {
		opts := rag.SearchOptions{AnchorNoteID: valueOrZero(req.NoteID)}
		searchResult, searchErr := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, opts)
		contexts := []rag.ChunkWithScore{}
		if searchErr == nil {
			contexts = searchResult.Results
		}
		answer, err := s.rag.Generate(ctx, buildAssistantRAGPrompt(req.Query, contexts, opts.AnchorNoteID))
		if err != nil {
			if searchErr != nil {
				writeErr(w, http.StatusBadGateway, searchErr)
				return
			}
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, rag.AskResult{
			Query:    req.Query,
			Answer:   answer,
			Contexts: contexts,
		})
		return
	}
	result, err := s.rag.AskWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
		AnchorNoteID: valueOrZero(req.NoteID),
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func buildLibraryRAGPrompt(query string, contexts []rag.ChunkWithScore, anchorNoteID int64) string {
	var sb strings.Builder
	sb.WriteString("你是全库问答助手，职责是回答用户关于自己笔记库的问题。\n")
	sb.WriteString("回答边界：必须基于用户问题中显式提供的材料和检索到的全库笔记片段回答；不要凭空补充笔记中没有的事实。\n")
	sb.WriteString("适合处理的问题包括：查找某个概念或结论、按主题总结多篇笔记、比较不同方案、梳理待继续推进的主题、指出证据不足或缺失的笔记。\n")
	sb.WriteString("如果上下文不足，先说明“笔记库里没有足够依据”，再给出可以继续搜索或补充的关键词。不要替用户写一段无来源的泛泛建议。\n")
	if anchorNoteID > 0 {
		sb.WriteString("当前打开笔记如出现在上下文中会标记为 CURRENT_NOTE；提到当前笔记时以 CURRENT_NOTE 为准。\n")
	}
	sb.WriteString("\n用户问题：\n")
	sb.WriteString(strings.TrimSpace(query))
	sb.WriteString("\n\n检索到的全库笔记片段：\n")
	if len(contexts) == 0 {
		sb.WriteString("无可用检索片段。\n")
	} else {
		for i, c := range contexts {
			role := "LIBRARY_NOTE"
			if anchorNoteID > 0 && c.NoteID == anchorNoteID {
				role = "CURRENT_NOTE"
			}
			title := strings.TrimSpace(c.NoteTitle)
			titlePart := ""
			if title != "" {
				titlePart = fmt.Sprintf(" title=%q", title)
			}
			sb.WriteString(fmt.Sprintf("[%d] %s note_id=%d%s chunk_index=%d score=%.4f\n%s\n\n",
				i+1, role, c.NoteID, titlePart, c.Index, c.Score, c.Content))
		}
	}
	sb.WriteString("请用中文回答。先直接回答问题，再用“依据”简要说明来自哪些笔记标题；除非用户要求，不要输出内部 chunk 编号。")
	return sb.String()
}

func buildAssistantRAGPrompt(query string, contexts []rag.ChunkWithScore, anchorNoteID int64) string {
	var sb strings.Builder
	sb.WriteString("你是独立的工作台 AI 助手，不只是单篇笔记里的问答功能。\n")
	sb.WriteString("你可以处理用户直接输入的计划、临时材料、附件内容，也可以参考检索到的笔记。\n")
	sb.WriteString("优先级：1) 用户问题里显式给出的内容和【助手工作台】上下文；2) 用户选择的附件或笔记；3) 下方检索到的全库笔记片段。\n")
	sb.WriteString("检索结果只是参考。不要因为检索上下文不足就直接回答“不知道”；只有当用户问题、显式上下文和检索结果都不足时，才说明缺口并给出下一步该补充什么。\n")
	sb.WriteString("如果用户要求整理计划，请直接提炼待办、排序、风险/注意事项、可推迟项，不要输出方法论模板。\n")
	if anchorNoteID > 0 {
		sb.WriteString("当前打开笔记如出现在上下文中会标记为 CURRENT_NOTE；提到当前笔记时以 CURRENT_NOTE 为准。\n")
	}
	sb.WriteString("\n用户请求：\n")
	sb.WriteString(strings.TrimSpace(query))
	sb.WriteString("\n\n可参考的全库笔记片段：\n")
	if len(contexts) == 0 {
		sb.WriteString("无可用检索片段。\n")
	} else {
		for i, c := range contexts {
			role := "RELATED_NOTE"
			if anchorNoteID > 0 && c.NoteID == anchorNoteID {
				role = "CURRENT_NOTE"
			}
			title := strings.TrimSpace(c.NoteTitle)
			titlePart := ""
			if title != "" {
				titlePart = fmt.Sprintf(" title=%q", title)
			}
			sb.WriteString(fmt.Sprintf("[%d] %s note_id=%d%s chunk_index=%d score=%.4f\n%s\n\n",
				i+1, role, c.NoteID, titlePart, c.Index, c.Score, c.Content))
		}
	}
	sb.WriteString("请用中文给出具体、可执行的回答。除非用户要求，否则不要输出内部检索编号。")
	return sb.String()
}

type optimizeNoteRequest struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

type optimizeReference struct {
	NoteID int64  `json:"note_id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
}

type optimizeContextNote struct {
	ID       int64
	Title    string
	Path     string
	Markdown string
}

func (s *Server) handleOptimizeNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if s.rag == nil {
		writeErrMsg(w, http.StatusServiceUnavailable, "ai optimizer not configured")
		return
	}

	var req optimizeNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	current, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(current.Title)
	}
	if title == "" {
		title = "Untitled"
	}

	markdown := req.Markdown
	if strings.TrimSpace(markdown) == "" {
		markdown = current.Markdown
	}

	refs, docs, err := s.collectOptimizeContext(ctx, id, markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	prompt := buildOptimizePrompt(title, markdown, docs)
	optimized, err := s.rag.Generate(ctx, prompt)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}

	optimized = unwrapMarkdownFence(optimized)
	if strings.TrimSpace(optimized) == "" {
		writeErrMsg(w, http.StatusBadGateway, "empty optimization result")
		return
	}

	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, optimized)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"markdown":   optimized,
		"html":       html,
		"references": refs,
	})
}

func (s *Server) renderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := s.md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *Server) resolveStoredInternalLinks(ctx context.Context, markdown string) (string, error) {
	notes, err := s.store.ListNotesLite(ctx, store.NoteFilter{IncludeArchived: true})
	if err != nil {
		return "", err
	}

	byID := make(map[int64]store.Note, len(notes))
	byTitle := make(map[string]int64, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
		key := strings.ToLower(strings.TrimSpace(note.Title))
		if key != "" {
			byTitle[key] = note.ID
		}
	}

	byPath := make(map[string]int64, len(notes))
	for _, note := range notes {
		key := normalizeLinkPath(noteRoutePath(note, byID))
		if key == "" {
			continue
		}
		byPath[key] = note.ID
	}

	resolved := wikiLinkRE.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := wikiLinkRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := strings.TrimSpace(sub[1])
		if name == "" {
			return match
		}
		id := byTitle[strings.ToLower(name)]
		if id == 0 {
			return match
		}
		return fmt.Sprintf("[%s](note://%d)", name, id)
	})

	resolved = markdownLinkRE.ReplaceAllStringFunc(resolved, func(match string) string {
		sub := markdownLinkRE.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		if sub[1] == "!" {
			return match
		}
		target := strings.TrimSpace(sub[3])
		if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "note://") {
			return match
		}
		if externalLinkRE.MatchString(target) {
			return match
		}
		id := byPath[normalizeLinkPath(target)]
		if id == 0 {
			return match
		}
		return fmt.Sprintf("[%s](note://%d)", sub[2], id)
	})

	return resolved, nil
}

func noteRoutePath(note store.Note, byID map[int64]store.Note) string {
	parts := make([]string, 0, 4)
	cur := note
	seen := map[int64]struct{}{}
	for {
		if _, ok := seen[cur.ID]; ok {
			break
		}
		seen[cur.ID] = struct{}{}
		title := strings.TrimSpace(cur.Title)
		if title != "" {
			parts = append(parts, title)
		}
		if cur.ParentID == nil {
			break
		}
		parent, ok := byID[*cur.ParentID]
		if !ok || !noteHasTag(parent, "folder") {
			break
		}
		cur = parent
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

func normalizeLinkPath(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		decoded, err := url.PathUnescape(part)
		if err == nil {
			part = decoded
		}
		out = append(out, strings.ToLower(part))
	}
	return strings.Join(out, "/")
}

func (s *Server) collectOptimizeContext(ctx context.Context, selfID int64, markdown string) ([]optimizeReference, []optimizeContextNote, error) {
	notes, err := s.store.ListNotes(ctx, store.NoteFilter{IncludeArchived: true})
	if err != nil {
		return nil, nil, err
	}

	byID := make(map[int64]store.Note, len(notes))
	byTitle := make(map[string]int64, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
		key := strings.ToLower(strings.TrimSpace(note.Title))
		if key != "" {
			byTitle[key] = note.ID
		}
	}
	byPath := make(map[string]int64, len(notes))
	for _, note := range notes {
		path := normalizeLinkPath(noteRoutePath(note, byID))
		if path != "" {
			byPath[path] = note.ID
		}
	}

	appendID := func(out []int64, seen map[int64]struct{}, id int64) []int64 {
		if id <= 0 || id == selfID {
			return out
		}
		if _, ok := seen[id]; ok {
			return out
		}
		if _, ok := byID[id]; !ok {
			return out
		}
		seen[id] = struct{}{}
		return append(out, id)
	}

	seen := map[int64]struct{}{}
	var ids []int64
	resolvedWiki := wikiLinkRE.FindAllStringSubmatch(markdown, -1)
	for _, sub := range resolvedWiki {
		if len(sub) < 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(sub[1]))
		if name == "" {
			continue
		}
		ids = appendID(ids, seen, byTitle[name])
		if len(ids) >= 8 {
			break
		}
	}

	if len(ids) < 8 {
		resolvedLinks := markdownLinkRE.FindAllStringSubmatch(markdown, -1)
		for _, sub := range resolvedLinks {
			if len(sub) < 4 || sub[1] == "!" {
				continue
			}
			target := strings.TrimSpace(sub[3])
			if target == "" || strings.HasPrefix(target, "#") {
				continue
			}
			var id int64
			switch {
			case strings.HasPrefix(target, "note://"):
				rawID := strings.TrimPrefix(target, "note://")
				id, _ = strconv.ParseInt(rawID, 10, 64)
			case externalLinkRE.MatchString(target):
				continue
			default:
				id = byPath[normalizeLinkPath(target)]
			}
			ids = appendID(ids, seen, id)
			if len(ids) >= 8 {
				break
			}
		}
	}

	refs := make([]optimizeReference, 0, len(ids))
	docs := make([]optimizeContextNote, 0, len(ids))
	for _, id := range ids {
		note := byID[id]
		title := strings.TrimSpace(note.Title)
		if title == "" {
			title = fmt.Sprintf("Untitled#%d", note.ID)
		}
		path := noteRoutePath(note, byID)
		refs = append(refs, optimizeReference{
			NoteID: note.ID,
			Title:  title,
			Path:   path,
		})
		docs = append(docs, optimizeContextNote{
			ID:       note.ID,
			Title:    title,
			Path:     path,
			Markdown: note.Markdown,
		})
	}
	return refs, docs, nil
}

func buildOptimizePrompt(title, markdown string, docs []optimizeContextNote) string {
	var sb strings.Builder
	sb.WriteString("你是中文笔记整理助手。你的任务是把一篇已有笔记整理成更易读、更清晰、更适合复习的 Markdown 笔记。\n")
	sb.WriteString("必须遵守以下规则：\n")
	sb.WriteString("1. 只能重组、润色和排版已有内容，不得捏造新事实。\n")
	sb.WriteString("2. 保留原有技术结论、代码片段、链接、内部链接和引用含义。\n")
	sb.WriteString("3. 优先通过标题、列表、表格、引用块、强调、分段来提升可读性。\n")
	sb.WriteString("4. 如果原文结构混乱，可以重排章节顺序，但不要删掉关键信息。\n")
	sb.WriteString("5. 输出必须是完整 Markdown 正文，不要解释，不要加```markdown代码围栏。\n\n")
	sb.WriteString("当前笔记标题：")
	sb.WriteString(title)
	sb.WriteString("\n\n当前笔记原文：\n")
	sb.WriteString(markdown)
	if len(docs) > 0 {
		sb.WriteString("\n\n以下是当前笔记中链接到的关联笔记，可作为整理时的补充上下文。你可以参考它们来改进结构和表达，但仍然不能凭空新增事实：\n")
		for i, doc := range docs {
			sb.WriteString(fmt.Sprintf("\n[关联笔记 %d]\n标题：%s\n路径：%s\n内容：\n%s\n", i+1, doc.Title, doc.Path, doc.Markdown))
		}
	}
	sb.WriteString("\n请直接输出优化后的完整 Markdown。")
	return sb.String()
}

func unwrapMarkdownFence(raw string) string {
	text := strings.TrimSpace(raw)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return text
	}
	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(first, "```") || last != "```" {
		return text
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}

func blocksToMarkdown(blocks []store.NoteBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		content := strings.TrimSpace(b.Content)
		if content == "" {
			continue
		}
		level := b.Level
		if level < 0 {
			level = 0
		}
		indent := strings.Repeat("  ", level)
		switch b.Type {
		case "heading1":
			parts = append(parts, "# "+content)
		case "todo":
			prefix := "- [ ] "
			if b.Checked {
				prefix = "- [x] "
			}
			parts = append(parts, indent+prefix+content)
		case "code":
			parts = append(parts, "```\n"+content+"\n```")
		case "quote":
			lines := strings.Split(content, "\n")
			for i := range lines {
				lines[i] = strings.Repeat("> ", level+1) + lines[i]
			}
			parts = append(parts, strings.Join(lines, "\n"))
		case "table":
			parts = append(parts, content)
		default:
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func normalizeBlockInputs(in []store.NoteBlockInput) []store.NoteBlockInput {
	if len(in) == 0 {
		return in
	}
	out := make([]store.NoteBlockInput, 0, len(in))
	prevLevel := 0
	for i, b := range in {
		level := b.Level
		if level < 0 {
			level = 0
		}
		if level > 6 {
			level = 6
		}
		if i == 0 {
			level = 0
		} else if level > prevLevel+1 {
			level = prevLevel + 1
		}
		b.Level = level
		prevLevel = level
		out = append(out, b)
	}
	return out
}

func parseMarkdownToBlocks(noteID int64, markdown string) []store.NoteBlock {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var (
		out []store.NoteBlock
		pos int
	)
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		add := func(typ, content string, checked bool) {
			out = append(out, store.NoteBlock{
				NoteID:   noteID,
				Position: pos,
				Level:    0,
				Type:     typ,
				Content:  content,
				Checked:  checked,
			})
			pos++
		}

		if strings.HasPrefix(line, "```") {
			i++
			var code []string
			for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
				code = append(code, lines[i])
				i++
			}
			if i < len(lines) {
				i++
			}
			add("code", strings.TrimSpace(strings.Join(code, "\n")), false)
			continue
		}
		if strings.HasPrefix(line, "# ") {
			add("heading1", strings.TrimSpace(strings.TrimPrefix(line, "# ")), false)
			i++
			continue
		}
		leadingSpaces := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		level := leadingSpaces / 2
		todoLine := strings.TrimLeft(lines[i], " ")
		if strings.HasPrefix(todoLine, "- [ ] ") || strings.HasPrefix(strings.ToLower(todoLine), "- [x] ") {
			checked := strings.HasPrefix(strings.ToLower(todoLine), "- [x] ")
			content := strings.TrimSpace(todoLine[6:])
			add("todo", content, checked)
			out[len(out)-1].Level = level
			i++
			continue
		}
		if strings.HasPrefix(line, "> ") {
			var quoteLines []string
			for i < len(lines) {
				cur := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(cur, "> ") {
					break
				}
				quoteLines = append(quoteLines, strings.TrimPrefix(cur, "> "))
				i++
			}
			add("quote", strings.Join(quoteLines, "\n"), false)
			continue
		}
		if strings.HasPrefix(line, "|") {
			var tableLines []string
			for i < len(lines) {
				cur := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(cur, "|") {
					break
				}
				tableLines = append(tableLines, lines[i])
				i++
			}
			add("table", strings.TrimSpace(strings.Join(tableLines, "\n")), false)
			continue
		}

		var para []string
		for i < len(lines) {
			cur := strings.TrimSpace(lines[i])
			if cur == "" {
				break
			}
			if strings.HasPrefix(cur, "# ") ||
				strings.HasPrefix(cur, "```") ||
				strings.HasPrefix(cur, "> ") ||
				strings.HasPrefix(cur, "|") ||
				strings.HasPrefix(cur, "- [ ] ") ||
				strings.HasPrefix(strings.ToLower(cur), "- [x] ") {
				break
			}
			para = append(para, lines[i])
			i++
		}
		add("paragraph", strings.TrimSpace(strings.Join(para, "\n")), false)
	}
	return out
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeErrMsg(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func parsePositiveURLInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeErrMsg(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func parseBool(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "1" || v == "true" || v == "yes"
}

func (s *Server) validateParentFolder(ctx context.Context, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.store.GetNote(ctx, *parentID)
	if err != nil {
		return err
	}
	if !noteHasTag(parent, "folder") {
		return errors.New("parent note must be a folder")
	}
	return nil
}

func noteHasTag(note store.Note, want string) bool {
	for _, tag := range note.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), want) {
			return true
		}
	}
	return false
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		if r == ' ' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func valueOrZero(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeErrMsg(w, status, err.Error())
}

func writeErrMsg(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("fileServer does not permit URL parameters.")
	}
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	fs := http.StripPrefix(strings.TrimRight(path, "/*"), http.FileServer(root))

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			writeErrMsg(w, http.StatusNotFound, "not found")
			return
		}

		if r.URL.Path == "/" || r.URL.Path == "" {
			http.ServeFile(w, r, "web/static/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})
}
