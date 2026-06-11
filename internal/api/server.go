package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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

// ---- helpers ----

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
