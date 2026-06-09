package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"note/internal/store"
)

type noteReference struct {
	ID    int64    `json:"id"`
	Title string   `json:"title"`
	Path  string   `json:"path"`
	Tags  []string `json:"tags,omitempty"`
}

type recommendation struct {
	Note   noteReference `json:"note"`
	Score  float64       `json:"score"`
	Reason string        `json:"reason"`
}

type externalResource struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Source      string `json:"source"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

type noteLinkReport struct {
	Outgoing         []noteReference `json:"outgoing"`
	Backlinks        []noteReference `json:"backlinks"`
	UnlinkedMentions []noteReference `json:"unlinked_mentions"`
}

type qualityIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

type flashcard struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Source   string `json:"source,omitempty"`
}

type noteInsights struct {
	Summary           string                 `json:"summary"`
	Outline           []string               `json:"outline"`
	Keywords          []string               `json:"keywords"`
	SuggestedTags     []string               `json:"suggested_tags"`
	QualityScore      int                    `json:"quality_score"`
	QualityIssues     []qualityIssue         `json:"quality_issues"`
	Flashcards        []store.ReviewQuestion `json:"flashcards"`
	Recommendations   []recommendation       `json:"recommendations"`
	Links             noteLinkReport         `json:"links"`
	DuplicateWarnings []recommendation       `json:"duplicate_warnings"`
	UsedAI            bool                   `json:"used_ai"`
}

type reviewReport struct {
	TodayUpdated     []noteReference  `json:"today_updated"`
	ReviewCandidates []recommendation `json:"review_candidates"`
	OrphanNotes      []noteReference  `json:"orphan_notes"`
	RecommendedNext  []recommendation `json:"recommended_next"`
}

type taskItem struct {
	NoteID    int64  `json:"note_id"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	Text      string `json:"text"`
	Checked   bool   `json:"checked"`
	Line      int    `json:"line"`
	UpdatedAt string `json:"updated_at"`
}

type noteTemplate struct {
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Tags     []string `json:"tags"`
	Markdown string   `json:"markdown"`
}

type aiRecommendationResponse struct {
	ID              int64              `json:"id,omitempty"`
	Topic           string             `json:"topic,omitempty"`
	Summary         string             `json:"summary"`
	Recommendations []recommendation   `json:"recommendations"`
	References      []noteReference    `json:"references"`
	Resources       []externalResource `json:"resources"`
	UsedAI          bool               `json:"used_ai"`
	CreatedAt       string             `json:"created_at,omitempty"`
}

type recommendationSessionHistoryItem struct {
	ID        int64                    `json:"id"`
	Topic     string                   `json:"topic"`
	Result    aiRecommendationResponse `json:"result"`
	CreatedAt string                   `json:"created_at"`
}

type weeklyReportResponse struct {
	Note    store.Note            `json:"note"`
	Sources []noteReference       `json:"sources"`
	Files   []weeklyFileReference `json:"files,omitempty"`
	UsedAI  bool                  `json:"used_ai"`
}

type weeklyFileSource struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type weeklyFileReference struct {
	Name string `json:"name"`
}

var headingRE = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
var todoRE = regexp.MustCompile(`(?m)^\s*-\s+\[([ xX])\]\s+(.+)$`)
var markdownLinkTargetRE = regexp.MustCompile(`!?\[[^\]]+\]\(([^)]+)\)`)
var weeklyNumberedNoteRefRE = regexp.MustCompile(`笔记\s*((?:\[\d+\]\s*)+)`)
var weeklyRefNumberRE = regexp.MustCompile(`\[(\d+)\]`)

func (s *Server) handleNoteRecommendations(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	current, ok := byID[id]
	if !ok {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	recs := buildRecommendations(current, notes, byID, 8, false)
	writeJSON(w, http.StatusOK, recs)
}

func (s *Server) handleNoteLinks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	current, ok := byID[id]
	if !ok {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	writeJSON(w, http.StatusOK, buildLinkReport(current, notes, byID))
}

func (s *Server) handleSuggestTags(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
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
	allTags, err := s.store.ListDistinctTags(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if tags, err := s.aiSuggestTags(ctx, n, allTags, 6); err == nil && len(tags) > 0 {
		writeJSON(w, http.StatusOK, tags)
		return
	}
	writeJSON(w, http.StatusOK, suggestTags(n, allTags, 6))
}

func (s *Server) handleNoteInsights(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("cached") == "1" || strings.EqualFold(r.URL.Query().Get("cached"), "true") {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		insight, err := s.store.GetNoteInsight(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			writeErrMsg(w, http.StatusNotFound, "insights not found")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(insight.Content))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	current, ok := byID[id]
	if !ok {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	tags, err := s.store.ListDistinctTags(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	links := buildLinkReport(current, notes, byID)
	recs := buildRecommendations(current, notes, byID, 6, false)
	duplicates := buildRecommendations(current, notes, byID, 5, true)
	issues, score := assessNoteQuality(current, links)
	questions, err := s.store.ListReviewQuestions(ctx, current.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	insights := buildLocalNoteInsights(current, tags, questions, recs, links, duplicates, issues, score)
	if aiInsights, err := s.aiNoteInsights(ctx, current, tags, insights); err == nil {
		insights = mergeAIInsights(insights, aiInsights, current, 6)
	}
	raw, err := json.Marshal(insights)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.store.SaveNoteInsight(ctx, current.ID, current.UpdatedAt, raw); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, insights)
}

func (s *Server) handleListReviewQuestions(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if _, err := s.store.GetNote(ctx, id); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	questions, err := s.store.ListReviewQuestions(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if questions == nil {
		questions = []store.ReviewQuestion{}
	}
	writeJSON(w, http.StatusOK, questions)
}

func (s *Server) handleCreateReviewQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	q, err := s.store.CreateReviewQuestion(ctx, id, store.ReviewQuestionInput{
		Question: req.Question,
		Answer:   req.Answer,
		Source:   "manual",
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "question is required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (s *Server) handleGenerateReviewQuestions(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Count int `json:"count"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Count <= 0 {
		req.Count = 3
	}
	if req.Count > 8 {
		req.Count = 8
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
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

	existingQuestions, err := s.store.ListReviewQuestions(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	seen := map[string]struct{}{}
	for _, item := range existingQuestions {
		seen[strings.ToLower(strings.TrimSpace(item.Question))] = struct{}{}
	}

	candidates := s.generateReviewQuestionCandidates(ctx, n, req.Count)
	created := make([]store.ReviewQuestion, 0, len(candidates))
	for _, card := range candidates {
		key := strings.ToLower(strings.TrimSpace(card.Question))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		q, err := s.store.CreateReviewQuestion(ctx, id, store.ReviewQuestionInput{
			Question: card.Question,
			Answer:   card.Answer,
			Source:   "ai",
		})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		created = append(created, q)
	}
	all, err := s.store.ListReviewQuestions(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"created": created,
		"items":   all,
	})
}

func (s *Server) handleUpdateReviewQuestion(w http.ResponseWriter, r *http.Request) {
	noteID, ok := parseID(w, r)
	if !ok {
		return
	}
	questionID, ok := parsePositiveURLInt(w, r, "questionID")
	if !ok {
		return
	}
	var req struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	q, err := s.store.UpdateReviewQuestion(ctx, noteID, questionID, store.ReviewQuestionInput{
		Question: req.Question,
		Answer:   req.Answer,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "review question not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "question is required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) handleDeleteReviewQuestion(w http.ResponseWriter, r *http.Request) {
	noteID, ok := parseID(w, r)
	if !ok {
		return
	}
	questionID, ok := parsePositiveURLInt(w, r, "questionID")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.store.DeleteReviewQuestion(ctx, noteID, questionID)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "review question not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDailyReview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	today := time.Now().Local().Format("2006-01-02")
	report := reviewReport{
		TodayUpdated:     []noteReference{},
		ReviewCandidates: []recommendation{},
		OrphanNotes:      orphanNotes(notes, byID, 8),
		RecommendedNext:  []recommendation{},
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].UpdatedAt.After(notes[j].UpdatedAt) })
	for _, note := range notes {
		if note.UpdatedAt.Local().Format("2006-01-02") == today && len(report.TodayUpdated) < 8 {
			report.TodayUpdated = append(report.TodayUpdated, toReference(note, byID))
		}
		age := time.Since(note.UpdatedAt)
		if age > 14*24*time.Hour && len(report.ReviewCandidates) < 8 {
			report.ReviewCandidates = append(report.ReviewCandidates, recommendation{
				Note:   toReference(note, byID),
				Score:  math.Min(age.Hours()/24/30, 1),
				Reason: fmt.Sprintf("约 %.0f 天未更新，适合回顾", age.Hours()/24),
			})
		}
		if len(report.RecommendedNext) < 8 {
			report.RecommendedNext = append(report.RecommendedNext, recommendation{
				Note:   toReference(note, byID),
				Score:  recencyScore(note.UpdatedAt),
				Reason: "最近活跃内容，可以继续补充",
			})
		}
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAIRecommendation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string  `json:"topic"`
		NoteIDs []int64 `json:"note_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" && len(req.NoteIDs) == 0 {
		writeErrMsg(w, http.StatusBadRequest, "topic or note_ids is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	_, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	selected := make([]store.Note, 0, len(req.NoteIDs))
	seen := make(map[int64]struct{})
	for _, id := range req.NoteIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		note, ok := byID[id]
		if !ok || noteHasTag(note, "folder") {
			continue
		}
		if full, err := s.store.GetNote(ctx, id); err == nil {
			note = full
		}
		selected = append(selected, note)
	}

	searchTopic := recommendationSearchTopic(topic, selected)
	resources := s.searchExternalResources(ctx, searchTopic, 8)

	summary, err := s.generateRecommendationSummary(ctx, topic, selected, resources, byID)
	usedAI := err == nil
	if err != nil {
		summary = fallbackRecommendationSummary(searchTopic, selected, resources)
	}
	refs := make([]noteReference, 0, len(selected))
	for _, note := range selected {
		refs = append(refs, toReference(note, byID))
	}
	resp := aiRecommendationResponse{
		Topic:           searchTopic,
		Summary:         summary,
		Recommendations: []recommendation{},
		References:      refs,
		Resources:       resources,
		UsedAI:          usedAI,
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	session, err := s.store.CreateRecommendationSession(ctx, searchTopic, raw)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.ID = session.ID
	resp.CreatedAt = session.CreatedAt.Format(time.RFC3339)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListRecommendationSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	sessions, err := s.store.ListRecommendationSessions(ctx, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]recommendationSessionHistoryItem, 0, len(sessions))
	for _, session := range sessions {
		item, err := recommendationHistoryItemFromStore(session)
		if err == nil {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteRecommendationSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := s.store.DeleteRecommendationSession(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "recommendation session not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func recommendationHistoryItemFromStore(session store.RecommendationSession) (recommendationSessionHistoryItem, error) {
	var result aiRecommendationResponse
	if err := json.Unmarshal([]byte(session.Result), &result); err != nil {
		return recommendationSessionHistoryItem{}, err
	}
	result.ID = session.ID
	result.CreatedAt = session.CreatedAt.Format(time.RFC3339)
	if strings.TrimSpace(result.Topic) == "" {
		result.Topic = session.Topic
	}
	return recommendationSessionHistoryItem{
		ID:        session.ID,
		Topic:     session.Topic,
		Result:    result,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *Server) handleCreateWeeklyReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string             `json:"title"`
		ParentID    *int64             `json:"parent_id"`
		NoteIDs     []int64            `json:"note_ids"`
		FileSources []weeklyFileSource `json:"file_sources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := s.validateParentFolder(ctx, req.ParentID); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusBadRequest, "parent note not found")
		return
	} else if err != nil {
		writeErrMsg(w, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "本周学习周报 " + time.Now().Format("2006-01-02")
	}

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	files := cleanWeeklyFileSources(req.FileSources, 8)
	sources := selectedWeeklySourceNotes(notes, req.NoteIDs)
	if len(req.NoteIDs) == 0 && len(files) == 0 {
		sources = weeklySourceNotes(notes, 10)
	}
	markdown, usedAI := s.generateWeeklyReportMarkdown(ctx, sources, files, byID)
	if !validWeeklyReportMarkdown(markdown) {
		markdown = fallbackWeeklyReportMarkdown(sources, files, byID)
		usedAI = false
	}
	markdown = normalizeWeeklyReportNoteReferences(markdown, sources)

	html, err := s.renderMarkdown(markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	note, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: req.ParentID,
		Title:    title,
		Markdown: markdown,
		HTML:     html,
		Tags:     []string{"weekly", "report", "ai-writing"},
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if s.rag != nil {
		_ = s.rag.IndexNote(ctx, note.ID, note.Markdown)
	}

	refs := make([]noteReference, 0, len(sources))
	for _, source := range sources {
		refs = append(refs, toReference(source, byID))
	}
	writeJSON(w, http.StatusCreated, weeklyReportResponse{
		Note:    note,
		Sources: refs,
		Files:   weeklyFileReferences(files),
		UsedAI:  usedAI,
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var tasks []taskItem
	for _, note := range notes {
		lines := strings.Split(strings.ReplaceAll(note.Markdown, "\r\n", "\n"), "\n")
		for i, line := range lines {
			m := todoRE.FindStringSubmatch(line)
			if len(m) < 3 {
				continue
			}
			tasks = append(tasks, taskItem{
				NoteID:    note.ID,
				Title:     displayTitle(note),
				Path:      noteRoutePath(note, byID),
				Text:      strings.TrimSpace(m[2]),
				Checked:   strings.EqualFold(m[1], "x"),
				Line:      i + 1,
				UpdatedAt: note.UpdatedAt.Format(time.RFC3339),
			})
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Checked != tasks[j].Checked {
			return !tasks[i].Checked
		}
		return tasks[i].UpdatedAt > tasks[j].UpdatedAt
	})
	if tasks == nil {
		tasks = []taskItem{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, builtInTemplates())
}

func (s *Server) handleCreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	var tpl noteTemplate
	for _, item := range builtInTemplates() {
		if item.Key == key {
			tpl = item
			break
		}
	}
	if tpl.Key == "" {
		writeErrMsg(w, http.StatusNotFound, "template not found")
		return
	}

	var req struct {
		Title    string `json:"title"`
		ParentID *int64 `json:"parent_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = tpl.Name
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
	html, err := s.renderMarkdown(tpl.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: req.ParentID,
		Title:    title,
		Markdown: tpl.Markdown,
		HTML:     html,
		Tags:     tpl.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.rag.IndexNote(ctx, n.ID, n.Markdown)
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleImportNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string   `json:"title"`
		Markdown string   `json:"markdown"`
		Text     string   `json:"text"`
		ParentID *int64   `json:"parent_id"`
		Tags     []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	markdown := strings.TrimSpace(req.Markdown)
	if markdown == "" {
		markdown = strings.TrimSpace(req.Text)
	}
	if markdown == "" {
		writeErrMsg(w, http.StatusBadRequest, "markdown or text is required")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = firstMeaningfulLine(markdown)
	}
	if title == "" {
		title = "Imported Note"
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
	html, err := s.renderMarkdown(markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: req.ParentID,
		Title:    strings.TrimPrefix(title, "# "),
		Markdown: markdown,
		HTML:     html,
		Tags:     req.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	_ = s.rag.IndexNote(ctx, n.ID, n.Markdown)
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) activeNotesWithIndex(ctx context.Context) ([]store.Note, map[int64]store.Note, error) {
	notes, err := s.store.ListNotes(ctx, store.NoteFilter{})
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[int64]store.Note, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
	}
	return notes, byID, nil
}

func buildRecommendations(current store.Note, notes []store.Note, byID map[int64]store.Note, limit int, duplicateMode bool) []recommendation {
	curTokens := tokenSet(current.Title + " " + current.Markdown)
	curTags := tagSet(current.Tags)
	linked := linkedNoteIDs(current, byID)
	var out []recommendation
	for _, note := range notes {
		if note.ID == current.ID || note.IsArchived {
			continue
		}
		score := 0.0
		reasons := make([]string, 0, 3)
		sharedTags := sharedStringCount(curTags, tagSet(note.Tags))
		if sharedTags > 0 {
			score += math.Min(float64(sharedTags)*0.22, 0.44)
			reasons = append(reasons, "共享标签")
		}
		if current.ParentID != nil && note.ParentID != nil && *current.ParentID == *note.ParentID {
			score += 0.18
			reasons = append(reasons, "同一文件夹")
		}
		if _, ok := linked[note.ID]; ok {
			score += 0.34
			reasons = append(reasons, "已有内部链接")
		}
		tokenScore := jaccard(curTokens, tokenSet(note.Title+" "+note.Markdown))
		score += tokenScore * 0.7
		if tokenScore > 0.12 {
			reasons = append(reasons, "内容相近")
		}
		score += recencyScore(note.UpdatedAt) * 0.08
		if duplicateMode && tokenScore < 0.24 {
			continue
		}
		if !duplicateMode && score < 0.08 {
			continue
		}
		reason := strings.Join(reasons, "、")
		if reason == "" {
			reason = "最近活跃"
		}
		if duplicateMode {
			reason = "内容相似，建议检查是否重复或合并"
		}
		out = append(out, recommendation{Note: toReference(note, byID), Score: roundScore(score), Reason: reason})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Note.ID > out[j].Note.ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildLinkReport(current store.Note, notes []store.Note, byID map[int64]store.Note) noteLinkReport {
	outgoingIDs := linkedNoteIDs(current, byID)
	backlinkIDs := map[int64]struct{}{}
	for _, note := range notes {
		if note.ID == current.ID {
			continue
		}
		if _, ok := linkedNoteIDs(note, byID)[current.ID]; ok {
			backlinkIDs[note.ID] = struct{}{}
		}
	}
	mentionedIDs := map[int64]struct{}{}
	lowerMarkdown := strings.ToLower(current.Markdown)
	for _, note := range notes {
		if note.ID == current.ID {
			continue
		}
		title := strings.TrimSpace(note.Title)
		if title == "" || utf8.RuneCountInString(title) < 2 {
			continue
		}
		if _, linked := outgoingIDs[note.ID]; linked {
			continue
		}
		if strings.Contains(lowerMarkdown, strings.ToLower(title)) {
			mentionedIDs[note.ID] = struct{}{}
		}
	}
	return noteLinkReport{
		Outgoing:         refsFromSet(outgoingIDs, byID),
		Backlinks:        refsFromSet(backlinkIDs, byID),
		UnlinkedMentions: refsFromSet(mentionedIDs, byID),
	}
}

func linkedNoteIDs(note store.Note, byID map[int64]store.Note) map[int64]struct{} {
	out := map[int64]struct{}{}
	byTitle := make(map[string]int64, len(byID))
	byPath := make(map[string]int64, len(byID))
	for _, n := range byID {
		if title := strings.ToLower(strings.TrimSpace(n.Title)); title != "" {
			byTitle[title] = n.ID
		}
		if path := normalizeLinkPath(noteRoutePath(n, byID)); path != "" {
			byPath[path] = n.ID
		}
	}
	for _, sub := range wikiLinkRE.FindAllStringSubmatch(note.Markdown, -1) {
		if len(sub) < 2 {
			continue
		}
		if id := byTitle[strings.ToLower(strings.TrimSpace(sub[1]))]; id > 0 {
			out[id] = struct{}{}
		}
	}
	for _, sub := range markdownLinkTargetRE.FindAllStringSubmatch(note.Markdown, -1) {
		if len(sub) < 2 {
			continue
		}
		target := strings.TrimSpace(sub[1])
		if strings.HasPrefix(target, "note://") {
			if id, err := strconv.ParseInt(strings.TrimPrefix(target, "note://"), 10, 64); err == nil && id > 0 {
				out[id] = struct{}{}
			}
			continue
		}
		if externalLinkRE.MatchString(target) || strings.HasPrefix(target, "#") {
			continue
		}
		if id := byPath[normalizeLinkPath(target)]; id > 0 {
			out[id] = struct{}{}
		}
	}
	return out
}

func suggestTags(note store.Note, allTags []string, limit int) []string {
	existing := tagSet(note.Tags)
	textTokens := tokenSet(note.Title + " " + note.Markdown)
	type candidate struct {
		tag   string
		score float64
	}
	var candidates []candidate
	for _, tag := range allTags {
		clean := strings.ToLower(strings.TrimSpace(tag))
		if !validSuggestedTag(clean, true) {
			continue
		}
		if _, ok := existing[clean]; ok {
			continue
		}
		score := 0.0
		for token := range textTokens {
			if token == clean || strings.Contains(token, clean) || strings.Contains(clean, token) {
				score += 1
			}
		}
		if score > 0 {
			candidates = append(candidates, candidate{tag: clean, score: score})
		}
	}
	for _, kw := range topKeywords(note.Title+" "+note.Markdown, limit*2) {
		clean := strings.ToLower(strings.TrimSpace(kw))
		if !validSuggestedTag(clean, false) {
			continue
		}
		if _, ok := existing[clean]; ok {
			continue
		}
		found := false
		for i := range candidates {
			if candidates[i].tag == clean {
				candidates[i].score += 0.8
				found = true
				break
			}
		}
		if !found {
			candidates = append(candidates, candidate{tag: clean, score: 0.8})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].tag < candidates[j].tag
	})
	out := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, c := range candidates {
		if _, ok := seen[c.tag]; ok {
			continue
		}
		seen[c.tag] = struct{}{}
		out = append(out, c.tag)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildLocalNoteInsights(
	note store.Note,
	tags []string,
	questions []store.ReviewQuestion,
	recs []recommendation,
	links noteLinkReport,
	duplicates []recommendation,
	issues []qualityIssue,
	score int,
) noteInsights {
	return noteInsights{
		Summary:           summarizeMarkdown(note.Markdown, 180),
		Outline:           extractOutline(note.Markdown, 8),
		Keywords:          insightKeywords(note.Title+" "+note.Markdown, 8),
		SuggestedTags:     suggestTags(note, tags, 6),
		QualityScore:      score,
		QualityIssues:     issues,
		Flashcards:        questions,
		Recommendations:   recs,
		Links:             links,
		DuplicateWarnings: duplicates,
		UsedAI:            false,
	}
}

type aiNoteInsightPayload struct {
	Summary       string         `json:"summary"`
	Outline       []string       `json:"outline"`
	Keywords      []string       `json:"keywords"`
	SuggestedTags []string       `json:"suggested_tags"`
	QualityScore  int            `json:"quality_score"`
	QualityIssues []qualityIssue `json:"quality_issues"`
}

func (s *Server) aiNoteInsights(ctx context.Context, note store.Note, allTags []string, local noteInsights) (aiNoteInsightPayload, error) {
	if s.rag == nil {
		return aiNoteInsightPayload{}, errors.New("rag not configured")
	}
	var sb strings.Builder
	sb.WriteString("你是个人知识库的智能洞察助手。请真正分析这篇笔记，并完成这些任务：摘要、大纲、关键词、标签建议、质量评分、整理建议。\n")
	sb.WriteString("只能基于笔记正文判断，不要编造正文没有的事实。标签必须像标签：短、稳定、可复用；不要输出完整句子、方法步骤或解释性短语。\n")
	sb.WriteString("只输出严格 JSON 对象，不要 Markdown 代码块。字段格式：\n")
	sb.WriteString(`{"summary":"一句具体摘要","outline":["一级要点"],"keywords":["关键词"],"suggested_tags":["tag"],"quality_score":0到100整数,"quality_issues":[{"type":"structure|depth|tags|links|title|evidence","severity":"low|medium|high","message":"具体问题","suggestion":"具体动作"}]}` + "\n")
	sb.WriteString("suggested_tags 最多 6 个，不要包含现有标签；中文标签 2 到 6 个字，英文标签 1 到 3 个词，可用小写和连字符。\n")
	if len(allTags) > 0 {
		sb.WriteString("可复用的已有标签：")
		sb.WriteString(strings.Join(dedupeStrings(allTags, 80), ", "))
		sb.WriteString("\n")
	}
	if len(note.Tags) > 0 {
		sb.WriteString("当前笔记已有标签：")
		sb.WriteString(strings.Join(note.Tags, ", "))
		sb.WriteString("\n")
	}
	if len(local.SuggestedTags) > 0 {
		sb.WriteString("本地初筛标签候选，仅供参考，不要照抄不合格短语：")
		sb.WriteString(strings.Join(local.SuggestedTags, ", "))
		sb.WriteString("\n")
	}
	sb.WriteString("本地质量初筛分数：")
	sb.WriteString(strconv.Itoa(local.QualityScore))
	sb.WriteString("\n\n笔记标题：")
	sb.WriteString(displayTitle(note))
	sb.WriteString("\n\n笔记正文：\n")
	sb.WriteString(truncateRunes(plainText(note.Markdown), 5000))

	raw, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return aiNoteInsightPayload{}, err
	}
	payload, err := parseAIInsightJSON(raw)
	if err != nil {
		return aiNoteInsightPayload{}, err
	}
	if strings.TrimSpace(payload.Summary) == "" && len(payload.SuggestedTags) == 0 && payload.QualityScore == 0 {
		return aiNoteInsightPayload{}, errors.New("empty ai insight")
	}
	return payload, nil
}

func (s *Server) aiSuggestTags(ctx context.Context, note store.Note, allTags []string, limit int) ([]string, error) {
	if s.rag == nil {
		return nil, errors.New("rag not configured")
	}
	if limit <= 0 {
		limit = 6
	}
	local := suggestTags(note, allTags, limit)
	var sb strings.Builder
	sb.WriteString("你是笔记标签助手。请根据笔记内容给出真正可复用的标签。\n")
	sb.WriteString("要求：标签必须短、稳定、像分类词；不要输出句子、步骤、观点、解释性短语。不要包含已有标签。\n")
	sb.WriteString(fmt.Sprintf("只输出 JSON 数组，最多 %d 个，例如 [\"rag\",\"投资纪律\",\"decision\"]。\n", limit))
	if len(allTags) > 0 {
		sb.WriteString("已有标签库：")
		sb.WriteString(strings.Join(dedupeStrings(allTags, 80), ", "))
		sb.WriteString("\n")
	}
	if len(note.Tags) > 0 {
		sb.WriteString("当前笔记已有标签：")
		sb.WriteString(strings.Join(note.Tags, ", "))
		sb.WriteString("\n")
	}
	if len(local) > 0 {
		sb.WriteString("本地候选，仅供参考：")
		sb.WriteString(strings.Join(local, ", "))
		sb.WriteString("\n")
	}
	sb.WriteString("\n笔记标题：")
	sb.WriteString(displayTitle(note))
	sb.WriteString("\n\n笔记正文：\n")
	sb.WriteString(truncateRunes(plainText(note.Markdown), 3500))

	raw, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	tags := parseAITagJSON(raw, note.Tags, limit)
	if len(tags) == 0 {
		return nil, errors.New("empty ai tags")
	}
	return tags, nil
}

func parseAIInsightJSON(raw string) (aiNoteInsightPayload, error) {
	text := extractJSONObject(raw)
	var payload aiNoteInsightPayload
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return aiNoteInsightPayload{}, err
	}
	return payload, nil
}

func mergeAIInsights(local noteInsights, ai aiNoteInsightPayload, note store.Note, tagLimit int) noteInsights {
	out := local
	if summary := truncateRunes(strings.TrimSpace(ai.Summary), 260); summary != "" {
		out.Summary = summary
	}
	if outline := sanitizeStringList(ai.Outline, 8, 80); len(outline) > 0 {
		out.Outline = outline
	}
	if keywords := sanitizeStringList(ai.Keywords, 8, 32); len(keywords) > 0 {
		out.Keywords = keywords
	}
	if tags := sanitizeSuggestedTags(ai.SuggestedTags, note.Tags, tagLimit); len(tags) > 0 {
		out.SuggestedTags = tags
	}
	if ai.QualityScore > 0 {
		out.QualityScore = clampInt(ai.QualityScore, 0, 100)
	}
	if issues := sanitizeQualityIssues(ai.QualityIssues, 6); len(issues) > 0 {
		out.QualityIssues = issues
	}
	out.UsedAI = true
	return out
}

func parseAITagJSON(raw string, existing []string, limit int) []string {
	text := strings.TrimSpace(unwrapMarkdownFence(raw))
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		text = text[start : end+1]
	}
	var payload []string
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	return sanitizeSuggestedTags(payload, existing, limit)
}

func extractJSONObject(raw string) string {
	text := strings.TrimSpace(unwrapMarkdownFence(raw))
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func sanitizeStringList(items []string, limit, maxRunes int) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		clean := truncateRunes(strings.TrimSpace(item), maxRunes)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sanitizeSuggestedTags(items []string, existing []string, limit int) []string {
	if limit <= 0 {
		limit = 6
	}
	existingSet := tagSet(existing)
	out := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, item := range items {
		clean := strings.ToLower(strings.TrimSpace(item))
		clean = strings.Trim(clean, "#+ ")
		if !validSuggestedTag(clean, false) {
			continue
		}
		if _, ok := existingSet[clean]; ok {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sanitizeQualityIssues(items []qualityIssue, limit int) []qualityIssue {
	out := make([]qualityIssue, 0, len(items))
	for _, item := range items {
		msg := truncateRunes(strings.TrimSpace(item.Message), 120)
		suggestion := truncateRunes(strings.TrimSpace(item.Suggestion), 120)
		if msg == "" && suggestion == "" {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(item.Type))
		if typ == "" {
			typ = "ai"
		}
		severity := strings.ToLower(strings.TrimSpace(item.Severity))
		if severity != "high" && severity != "medium" && severity != "low" {
			severity = "medium"
		}
		out = append(out, qualityIssue{
			Type:       typ,
			Severity:   severity,
			Message:    msg,
			Suggestion: suggestion,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func validSuggestedTag(tag string, existingTag bool) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return false
	}
	runeCount := utf8.RuneCountInString(tag)
	if runeCount < 2 {
		return false
	}
	if strings.ContainsAny(tag, "，。！？；：、,.!?;:()（）[]【】{}<>《》\"'“”‘’/\\|") {
		return false
	}
	if strings.HasPrefix(tag, "-") || strings.HasSuffix(tag, "-") || strings.Contains(tag, "--") {
		return false
	}
	if existingTag {
		return runeCount <= 32
	}
	if containsCJK(tag) {
		if runeCount > 6 {
			return false
		}
		return !containsBadChinesePhrase(tag)
	}
	if runeCount > 24 {
		return false
	}
	parts := strings.FieldsFunc(tag, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	if len(parts) > 3 {
		return false
	}
	return true
}

func insightKeywords(text string, limit int) []string {
	raw := topKeywords(text, limit*3)
	out := make([]string, 0, limit)
	seen := map[string]struct{}{}
	for _, kw := range raw {
		clean := strings.TrimSpace(kw)
		if clean == "" {
			continue
		}
		if containsCJK(clean) {
			runeCount := utf8.RuneCountInString(clean)
			if runeCount > 10 || containsBadChinesePhrase(clean) {
				continue
			}
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func containsBadChinesePhrase(text string) bool {
	badParts := []string{
		"一个", "一种", "一组", "这个", "那个", "可以", "需要", "应该", "如何", "什么", "为什么",
		"不是", "而是", "也是", "同时", "通常", "包含", "因为", "所以", "是否", "避免", "读者",
		"用户", "团队", "市场", "可能", "时候", "如果", "例如", "这些", "那些", "可被", "进行", "即使", "很多",
		"实际工作", "主题常常", "方法通常", "后续", "形成", "不是为了", "先把", "再定义", "最后",
	}
	for _, part := range badParts {
		if strings.Contains(text, part) {
			return true
		}
	}
	return false
}

func containsCJK(text string) bool {
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			return true
		}
	}
	return false
}

func assessNoteQuality(note store.Note, links noteLinkReport) ([]qualityIssue, int) {
	var issues []qualityIssue
	add := func(typ, severity, message, suggestion string) {
		issues = append(issues, qualityIssue{Type: typ, Severity: severity, Message: message, Suggestion: suggestion})
	}
	plain := plainText(note.Markdown)
	if utf8.RuneCountInString(strings.TrimSpace(note.Title)) < 3 {
		add("title", "medium", "标题信息量偏少", "换成能说明主题的标题")
	}
	if utf8.RuneCountInString(plain) < 80 {
		add("depth", "high", "正文内容偏短", "补充背景、结论和关键依据")
	}
	if len(extractOutline(note.Markdown, 10)) == 0 && utf8.RuneCountInString(plain) > 180 {
		add("structure", "medium", "长笔记缺少标题结构", "加入二级标题或列表，让内容更容易复习")
	}
	if len(note.Tags) == 0 {
		add("tags", "medium", "缺少标签", "添加主题、项目或状态标签")
	}
	if len(links.Outgoing) == 0 && len(links.Backlinks) == 0 && utf8.RuneCountInString(plain) > 120 {
		add("links", "low", "这篇笔记暂时是知识孤岛", "关联到已有页面或补充反向链接")
	}
	score := 100
	for _, issue := range issues {
		switch issue.Severity {
		case "high":
			score -= 22
		case "medium":
			score -= 14
		default:
			score -= 8
		}
	}
	if score < 35 {
		score = 35
	}
	return issues, score
}

func summarizeMarkdown(markdown string, limit int) string {
	text := plainText(markdown)
	if text == "" {
		return "暂无可摘要内容。"
	}
	sentences := splitSentences(text)
	var picked []string
	for _, s := range sentences {
		if utf8.RuneCountInString(strings.Join(picked, ""))+utf8.RuneCountInString(s) > limit && len(picked) > 0 {
			break
		}
		picked = append(picked, s)
		if len(picked) >= 3 {
			break
		}
	}
	return truncateRunes(strings.Join(picked, " "), limit)
}

func extractOutline(markdown string, limit int) []string {
	var out []string
	for _, sub := range headingRE.FindAllStringSubmatch(markdown, -1) {
		if len(sub) < 2 {
			continue
		}
		out = append(out, strings.TrimSpace(sub[1]))
		if len(out) >= limit {
			break
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if utf8.RuneCountInString(line) >= 8 {
			out = append(out, truncateRunes(line, 48))
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildFlashcards(note store.Note, limit int) []flashcard {
	var cards []flashcard
	for _, heading := range extractOutline(note.Markdown, limit) {
		cards = append(cards, flashcard{
			Question: "这部分的核心内容是什么：" + heading + "？",
			Answer:   bestAnswerForHeading(note.Markdown, heading),
			Source:   displayTitle(note),
		})
		if len(cards) >= limit {
			return cards
		}
	}
	for _, kw := range topKeywords(note.Markdown, limit) {
		cards = append(cards, flashcard{
			Question: "请解释：" + kw,
			Answer:   "回到原笔记中定位该关键词，补充定义、背景和例子。",
			Source:   displayTitle(note),
		})
		if len(cards) >= limit {
			break
		}
	}
	return cards
}

func (s *Server) generateReviewQuestionCandidates(ctx context.Context, note store.Note, limit int) []flashcard {
	if limit <= 0 {
		limit = 3
	}
	if s.rag != nil {
		if cards, err := s.aiReviewQuestionCandidates(ctx, note, limit); err == nil && len(cards) > 0 {
			return cards
		}
	}
	cards := buildFlashcards(note, limit)
	if len(cards) > limit {
		return cards[:limit]
	}
	return cards
}

func (s *Server) aiReviewQuestionCandidates(ctx context.Context, note store.Note, limit int) ([]flashcard, error) {
	var sb strings.Builder
	sb.WriteString("你是复习问题设计助手。请根据用户的一篇笔记生成适合主动回忆的复习问题。\n")
	sb.WriteString("要求：问题具体、可回答、覆盖不同知识点；答案提示只能基于笔记内容，不要编造外部信息。\n")
	sb.WriteString("只输出 JSON 数组，不要 Markdown 代码围栏。数组元素格式为 {\"question\":\"...\",\"answer\":\"...\"}。\n")
	sb.WriteString(fmt.Sprintf("最多生成 %d 个。\n\n", limit))
	sb.WriteString("笔记标题：")
	sb.WriteString(displayTitle(note))
	sb.WriteString("\n\n笔记正文：\n")
	sb.WriteString(truncateRunes(plainText(note.Markdown), 3000))

	raw, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	cards := parseReviewQuestionJSON(raw, limit)
	if len(cards) == 0 {
		return nil, errors.New("empty ai review questions")
	}
	return cards, nil
}

func parseReviewQuestionJSON(raw string, limit int) []flashcard {
	text := strings.TrimSpace(unwrapMarkdownFence(raw))
	var payload []struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		start := strings.Index(text, "[")
		end := strings.LastIndex(text, "]")
		if start < 0 || end <= start {
			return nil
		}
		if err := json.Unmarshal([]byte(text[start:end+1]), &payload); err != nil {
			return nil
		}
	}
	seen := map[string]struct{}{}
	cards := make([]flashcard, 0, len(payload))
	for _, item := range payload {
		question := strings.TrimSpace(item.Question)
		if question == "" {
			continue
		}
		if _, ok := seen[question]; ok {
			continue
		}
		seen[question] = struct{}{}
		answer := strings.TrimSpace(item.Answer)
		if answer == "" {
			answer = "回到原笔记中定位相关段落，用自己的话回答。"
		}
		cards = append(cards, flashcard{
			Question: truncateRunes(question, 180),
			Answer:   truncateRunes(answer, 240),
			Source:   "ai",
		})
		if len(cards) >= limit {
			break
		}
	}
	return cards
}

func bestAnswerForHeading(markdown, heading string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, heading) {
			var parts []string
			for j := i + 1; j < len(lines) && len(parts) < 3; j++ {
				cur := strings.TrimSpace(lines[j])
				if cur == "" {
					continue
				}
				if strings.HasPrefix(cur, "#") {
					break
				}
				parts = append(parts, strings.TrimPrefix(cur, "- "))
			}
			if len(parts) > 0 {
				return truncateRunes(strings.Join(parts, " "), 140)
			}
		}
	}
	return "可以从这部分的要点、定义和例子进行回答。"
}

func orphanNotes(notes []store.Note, byID map[int64]store.Note, limit int) []noteReference {
	var out []noteReference
	backlinked := map[int64]struct{}{}
	for _, note := range notes {
		for id := range linkedNoteIDs(note, byID) {
			backlinked[id] = struct{}{}
		}
	}
	for _, note := range notes {
		if len(linkedNoteIDs(note, byID)) > 0 {
			continue
		}
		if _, ok := backlinked[note.ID]; ok {
			continue
		}
		out = append(out, toReference(note, byID))
		if len(out) >= limit {
			break
		}
	}
	return out
}

func builtInTemplates() []noteTemplate {
	return []noteTemplate{
		{Key: "study", Name: "学习笔记", Tags: []string{"study"}, Markdown: "# 主题\n\n## 核心概念\n\n- \n\n## 例子\n\n- \n\n## 复习问题\n\n- [ ] "},
		{Key: "meeting", Name: "会议记录", Tags: []string{"meeting"}, Markdown: "# 会议记录\n\n## 结论\n\n- \n\n## 待办\n\n- [ ] \n\n## 讨论\n\n- "},
		{Key: "project", Name: "项目日志", Tags: []string{"project"}, Markdown: "# 项目日志\n\n## 今日进展\n\n- \n\n## 风险\n\n- \n\n## 下一步\n\n- [ ] "},
		{Key: "paper", Name: "论文阅读", Tags: []string{"paper"}, Markdown: "# 论文阅读\n\n## 问题\n\n- \n\n## 方法\n\n- \n\n## 结论\n\n- \n\n## 可复用点\n\n- "},
		{Key: "bug", Name: "Bug 记录", Tags: []string{"bug"}, Markdown: "# Bug 记录\n\n## 现象\n\n- \n\n## 原因\n\n- \n\n## 修复\n\n- [ ] \n\n## 验证\n\n- [ ] "},
		{Key: "weekly", Name: "周报", Tags: []string{"weekly"}, Markdown: "# 周报\n\n## 本周完成\n\n- \n\n## 问题与风险\n\n- \n\n## 下周计划\n\n- [ ] "},
		{Key: "reading", Name: "读书笔记", Tags: []string{"reading"}, Markdown: "# 读书笔记\n\n## 摘要\n\n- \n\n## 触动我的观点\n\n- \n\n## 行动\n\n- [ ] "},
	}
}

var ddgResultRE = regexp.MustCompile(`(?s)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
var htmlTagRE = regexp.MustCompile(`(?s)<[^>]+>`)

func recommendationSearchTopic(topic string, selected []store.Note) string {
	if text := strings.TrimSpace(topic); text != "" {
		return text
	}
	var parts []string
	for _, note := range selected {
		if title := strings.TrimSpace(displayTitle(note)); title != "" {
			parts = append(parts, title)
		}
		for _, tag := range note.Tags {
			if tag = strings.TrimSpace(tag); tag != "" && !strings.EqualFold(tag, "folder") {
				parts = append(parts, tag)
			}
		}
		if len(parts) >= 6 {
			break
		}
	}
	if len(parts) == 0 {
		return "知识管理 学习资源"
	}
	return strings.Join(parts, " ")
}

func (s *Server) searchExternalResources(ctx context.Context, topic string, limit int) []externalResource {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		topic = "知识管理 学习资源"
	}
	searchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	seen := make(map[string]struct{})
	var resources []externalResource
	queries := []struct {
		text string
		kind string
	}{
		{topic + " article tutorial guide", "文章"},
		{topic + " site:youtube.com/watch", "视频"},
	}
	for _, query := range queries {
		for _, item := range searchDuckDuckGo(searchCtx, query.text, query.kind, limit) {
			if item.URL == "" {
				continue
			}
			key := strings.ToLower(item.URL)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			resources = append(resources, item)
			if len(resources) >= limit {
				return resources
			}
		}
	}
	if len(resources) < 3 {
		for _, item := range fallbackExternalResources(topic) {
			key := strings.ToLower(item.URL)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			resources = append(resources, item)
			if len(resources) >= limit {
				break
			}
		}
	}
	return resources
}

func searchDuckDuckGo(ctx context.Context, query, kind string, limit int) []externalResource {
	endpoint := "https://duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NoteWorkbench/1.0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil
	}
	matches := ddgResultRE.FindAllStringSubmatch(string(body), limit*2)
	out := make([]externalResource, 0, limit)
	for _, m := range matches {
		rawURL := cleanSearchResultURL(m[1])
		if rawURL == "" {
			continue
		}
		title := cleanHTMLText(m[2])
		if title == "" {
			title = rawURL
		}
		out = append(out, externalResource{
			Title:  title,
			URL:    rawURL,
			Source: resourceSource(rawURL),
			Kind:   resourceKind(rawURL, kind),
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

func cleanSearchResultURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//duckduckgo.com/l/?") || strings.HasPrefix(raw, "https://duckduckgo.com/l/?") {
		if strings.HasPrefix(raw, "//") {
			raw = "https:" + raw
		}
		u, err := url.Parse(raw)
		if err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				raw = target
			}
		}
	}
	if strings.HasPrefix(raw, "/l/?") {
		u, err := url.Parse("https://duckduckgo.com" + raw)
		if err == nil {
			if target := u.Query().Get("uddg"); target != "" {
				raw = target
			}
		}
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return ""
	}
	return raw
}

func cleanHTMLText(raw string) string {
	text := htmlTagRE.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func resourceSource(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "互联网"
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	return host
}

func resourceKind(raw, fallback string) string {
	host := resourceSource(raw)
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") || strings.Contains(host, "bilibili.com") {
		return "视频"
	}
	if strings.Contains(host, "arxiv.org") || strings.Contains(host, "scholar.google") {
		return "论文"
	}
	if fallback != "" {
		return fallback
	}
	return "文章"
}

func fallbackExternalResources(topic string) []externalResource {
	q := url.QueryEscape(topic)
	return []externalResource{
		{Title: "Google Scholar 论文搜索：" + topic, URL: "https://scholar.google.com/scholar?q=" + q, Source: "Google Scholar", Kind: "论文", Description: "用于查找论文、综述和引用脉络。"},
		{Title: "YouTube 视频搜索：" + topic, URL: "https://www.youtube.com/results?search_query=" + q, Source: "YouTube", Kind: "视频", Description: "用于查找课程、演讲和实践演示。"},
		{Title: "Bilibili 视频搜索：" + topic, URL: "https://search.bilibili.com/all?keyword=" + q, Source: "Bilibili", Kind: "视频", Description: "用于查找中文讲解、课程和项目演示。"},
		{Title: "DuckDuckGo 文章搜索：" + topic, URL: "https://duckduckgo.com/?q=" + q, Source: "DuckDuckGo", Kind: "文章", Description: "用于继续检索外部文章、教程和案例。"},
	}
}

func (s *Server) generateRecommendationSummary(ctx context.Context, topic string, selected []store.Note, resources []externalResource, byID map[int64]store.Note) (string, error) {
	if s.rag == nil {
		return "", errors.New("ai recommender not configured")
	}
	var sb strings.Builder
	sb.WriteString("你是一个互联网学习资源推荐助手。用户需要外部文章、视频、论文或教程链接，而不是推荐笔记系统里的内容。\n")
	sb.WriteString("请只围绕下方外部资源生成推荐总结；用户选择的本地笔记只能作为理解兴趣和背景的上下文，不要把本地笔记列为推荐内容。\n\n")
	if strings.TrimSpace(topic) != "" {
		sb.WriteString("用户希望被推荐的主题：\n")
		sb.WriteString(topic)
		sb.WriteString("\n\n")
	}
	if len(selected) > 0 {
		sb.WriteString("用户选择的参考笔记：\n")
		for i, note := range selected {
			sb.WriteString(fmt.Sprintf("[%d] %s\n路径：%s\n标签：%s\n内容摘录：\n%s\n\n",
				i+1,
				displayTitle(note),
				noteRoutePath(note, byID),
				strings.Join(note.Tags, ", "),
				truncateRunes(plainText(note.Markdown), 1000),
			))
		}
	}
	if len(resources) > 0 {
		sb.WriteString("已经检索到的外部资源：\n")
		for i, item := range resources {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n链接：%s\n来源：%s\n说明：%s\n\n",
				i+1,
				item.Kind,
				item.Title,
				item.URL,
				item.Source,
				item.Description,
			))
		}
	}
	sb.WriteString("请用中文输出，包含三个小节：\n")
	sb.WriteString("1. AI 总结：概括这个主题值得学习的关键问题。\n")
	sb.WriteString("2. 推荐资源：从外部资源中挑选 5 条，必须包含链接，并说明文章/视频/论文各自适合解决什么问题。\n")
	sb.WriteString("3. 下一步：给出 3 条学习和沉淀到笔记系统的行动。\n")
	return s.rag.Generate(ctx, sb.String())
}

func fallbackRecommendationSummary(topic string, selected []store.Note, resources []externalResource) string {
	var sb strings.Builder
	if strings.TrimSpace(topic) != "" {
		sb.WriteString("AI 总结\n")
		sb.WriteString("当前外部检索主题是：")
		sb.WriteString(strings.TrimSpace(topic))
		sb.WriteString("。下面优先给出互联网文章、视频或论文入口，供继续学习和整理。\n\n")
	}
	if len(selected) > 0 {
		sb.WriteString("参考上下文\n")
		for _, note := range selected {
			sb.WriteString("- ")
			sb.WriteString(displayTitle(note))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("推荐资源\n")
	if len(resources) == 0 {
		sb.WriteString("- 暂时没有拿到可用外部资源链接，可以换一个更具体的主题再试。\n")
	} else {
		for _, item := range resources {
			sb.WriteString("- ")
			sb.WriteString("[")
			sb.WriteString(item.Kind)
			sb.WriteString("] ")
			sb.WriteString(item.Title)
			sb.WriteString("：")
			sb.WriteString(item.URL)
			if item.Description != "" {
				sb.WriteString("。")
				sb.WriteString(item.Description)
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n下一步\n- 先打开 2 篇文章和 1 个视频，比较它们对核心概念的解释。\n- 把高频概念整理成一份术语表。\n- 记录每个资源解决的问题、可复用方法和仍待验证的疑问。\n")
	return sb.String()
}

func (s *Server) generateWeeklyReportMarkdown(ctx context.Context, sources []store.Note, files []weeklyFileSource, byID map[int64]store.Note) (string, bool) {
	if s.rag == nil {
		return fallbackWeeklyReportMarkdown(sources, files, byID), false
	}
	var sb strings.Builder
	sb.WriteString("你是一个学习周报写作助手。请根据用户本周更新和整理过的笔记，生成一篇 Markdown 学习周报。\n")
	sb.WriteString("必须包含这些二级标题：本周学习大纲、重点理解、下周学习建议、资源推荐。\n")
	sb.WriteString("要求：内容具体，资源推荐要给出推荐理由；如果资料不足，要基于已有笔记诚实总结，不要虚构实时网页来源。\n\n")
	sb.WriteString("引用笔记库内容时，必须写成用户可读的双链内部链接，例如 [[笔记标题]]。不要输出 note://123、[1]、[2]、笔记[1] 这类用户看不懂的编号。\n\n")
	if len(sources) == 0 && len(files) == 0 {
		sb.WriteString("本周没有明显更新的笔记，请生成一份通用但可执行的学习周报模板。\n")
	}
	if len(sources) > 0 {
		sb.WriteString("本周参考笔记：\n")
		for i, note := range sources {
			sb.WriteString(fmt.Sprintf("来源 %d\n笔记链接：%s\n路径：%s\n标签：%s\n摘要：%s\n\n",
				i+1,
				weeklyNoteMarkdownLink(note),
				noteRoutePath(note, byID),
				strings.Join(note.Tags, ", "),
				truncateRunes(plainText(note.Markdown), 700),
			))
		}
	}
	if len(files) > 0 {
		sb.WriteString("用户选择的本机文件参考材料：\n")
		for i, file := range files {
			sb.WriteString(fmt.Sprintf("本机文件 %d\n文件名：%s\n内容摘录：%s\n\n",
				i+1,
				file.Name,
				truncateRunes(plainText(file.Content), 1200),
			))
		}
	}
	out, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return fallbackWeeklyReportMarkdown(sources, files, byID), false
	}
	return strings.TrimSpace(out), true
}

func normalizeWeeklyReportNoteReferences(markdown string, sources []store.Note) string {
	if len(sources) == 0 || strings.TrimSpace(markdown) == "" {
		return markdown
	}
	return weeklyNumberedNoteRefRE.ReplaceAllStringFunc(markdown, func(match string) string {
		links := []string{}
		for _, sub := range weeklyRefNumberRE.FindAllStringSubmatch(match, -1) {
			if len(sub) < 2 {
				continue
			}
			idx, err := strconv.Atoi(sub[1])
			if err != nil || idx < 1 || idx > len(sources) {
				continue
			}
			links = append(links, weeklyNoteMarkdownLink(sources[idx-1]))
		}
		if len(links) == 0 {
			return match
		}
		return "笔记 " + strings.Join(links, "、")
	})
}

func weeklyNoteMarkdownLink(note store.Note) string {
	return fmt.Sprintf("[[%s]]", escapeWikiLinkLabel(displayTitle(note)))
}

func escapeWikiLinkLabel(label string) string {
	return strings.NewReplacer("[[", "", "]]", "").Replace(label)
}

func cleanWeeklyFileSources(files []weeklyFileSource, limit int) []weeklyFileSource {
	capacity := len(files)
	if capacity > limit {
		capacity = limit
	}
	out := make([]weeklyFileSource, 0, capacity)
	seen := map[string]struct{}{}
	for _, file := range files {
		if len(out) >= limit {
			break
		}
		content := strings.TrimSpace(file.Content)
		if content == "" {
			continue
		}
		name := strings.TrimSpace(file.Name)
		if name == "" {
			name = fmt.Sprintf("本机文件 %d", len(out)+1)
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, weeklyFileSource{
			Name:    name,
			Content: truncateRunes(content, 20000),
		})
	}
	return out
}

func weeklyFileReferences(files []weeklyFileSource) []weeklyFileReference {
	if len(files) == 0 {
		return nil
	}
	out := make([]weeklyFileReference, 0, len(files))
	for _, file := range files {
		out = append(out, weeklyFileReference{Name: file.Name})
	}
	return out
}

func selectedWeeklySourceNotes(notes []store.Note, ids []int64) []store.Note {
	if len(ids) == 0 {
		return nil
	}
	byID := make(map[int64]store.Note, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
	}
	out := make([]store.Note, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		note, ok := byID[id]
		if !ok || noteHasTag(note, "folder") {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, note)
	}
	return out
}

func weeklySourceNotes(notes []store.Note, limit int) []store.Note {
	var content []store.Note
	for _, note := range notes {
		if noteHasTag(note, "folder") {
			continue
		}
		content = append(content, note)
	}
	sort.Slice(content, func(i, j int) bool {
		return content[i].UpdatedAt.After(content[j].UpdatedAt)
	})
	cutoff := time.Now().AddDate(0, 0, -7)
	var recent []store.Note
	for _, note := range content {
		if note.UpdatedAt.After(cutoff) || note.CreatedAt.After(cutoff) {
			recent = append(recent, note)
			if len(recent) >= limit {
				return recent
			}
		}
	}
	if len(recent) > 0 {
		return recent
	}
	if len(content) > limit {
		return content[:limit]
	}
	return content
}

func validWeeklyReportMarkdown(markdown string) bool {
	text := strings.TrimSpace(markdown)
	if utf8.RuneCountInString(text) < 80 {
		return false
	}
	required := []string{"本周学习大纲", "下周学习建议", "资源推荐"}
	for _, item := range required {
		if !strings.Contains(text, item) {
			return false
		}
	}
	return true
}

func fallbackWeeklyReportMarkdown(sources []store.Note, files []weeklyFileSource, byID map[int64]store.Note) string {
	var sb strings.Builder
	sb.WriteString("# 本周学习周报\n\n")
	sb.WriteString("## 本周学习大纲\n\n")
	if len(sources) == 0 && len(files) == 0 {
		sb.WriteString("- 本周暂无明显更新的笔记，可以先从知识体检中心挑选一批待整理内容。\n")
	} else {
		for _, note := range sources {
			sb.WriteString("- ")
			sb.WriteString(weeklyNoteMarkdownLink(note))
			if path := noteRoutePath(note, byID); path != "" {
				sb.WriteString("：")
				sb.WriteString(path)
			}
			sb.WriteString("\n")
		}
		for _, file := range files {
			sb.WriteString("- 本机文件：")
			sb.WriteString(file.Name)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## 重点理解\n\n")
	if len(sources) == 0 && len(files) == 0 {
		sb.WriteString("- 先补齐本周学习主题，再沉淀成可复盘的笔记结构。\n")
	} else {
		for _, note := range sources {
			summary := summarizeMarkdown(note.Markdown, 120)
			sb.WriteString("- ")
			sb.WriteString(weeklyNoteMarkdownLink(note))
			sb.WriteString("：")
			sb.WriteString(summary)
			sb.WriteString("\n")
		}
		for _, file := range files {
			sb.WriteString("- ")
			sb.WriteString(file.Name)
			sb.WriteString("：")
			sb.WriteString(summarizeMarkdown(file.Content, 120))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n## 下周学习建议\n\n")
	sb.WriteString("- 选择 1 个核心主题继续深入，避免同时推进太多方向。\n")
	sb.WriteString("- 为本周新增或修改的笔记补充标签、二级标题和内部链接。\n")
	sb.WriteString("- 从任务中心拆出 3 个可执行动作，并为每个动作设置日期和优先级。\n")

	sb.WriteString("\n## 资源推荐\n\n")
	tags := weeklyTags(sources)
	if containsAny(tags, "go", "backend", "api") {
		sb.WriteString("- Go 官方文档与博客：适合继续补充后端工程、并发和运行时知识。\n")
	}
	if containsAny(tags, "rag", "llm", "search", "retrieval") {
		sb.WriteString("- RAG 论文与向量检索资料：适合把检索、召回和生成总结串成完整方案。\n")
	}
	if containsAny(tags, "frontend", "vue", "ux", "design") {
		sb.WriteString("- Vue 官方性能指南与可用性清单：适合继续优化界面响应和交互细节。\n")
	}
	if len(tags) == 0 || sb.String()[strings.LastIndex(sb.String(), "## 资源推荐"):] == "## 资源推荐\n\n" {
		sb.WriteString("- 选择一本与当前主线最相关的官方文档或经典教程，按章节整理成笔记。\n")
	}
	sb.WriteString("\n## 可执行清单\n\n")
	sb.WriteString("- [ ] 整理本周最重要的一篇笔记\n")
	sb.WriteString("- [ ] 为下周学习主题创建任务\n")
	sb.WriteString("- [ ] 使用推荐与回顾功能生成延伸阅读\n")
	return sb.String()
}

func weeklyTags(notes []store.Note) map[string]struct{} {
	out := map[string]struct{}{}
	for _, note := range notes {
		for _, tag := range note.Tags {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				out[tag] = struct{}{}
			}
		}
	}
	return out
}

func containsAny(tags map[string]struct{}, wants ...string) bool {
	for _, want := range wants {
		if _, ok := tags[strings.ToLower(want)]; ok {
			return true
		}
	}
	return false
}

func keywordTags(text string) []string {
	tokens := tokenizeText(text)
	if len(tokens) > 5 {
		tokens = tokens[:5]
	}
	return tokens
}

func refsFromSet(ids map[int64]struct{}, byID map[int64]store.Note) []noteReference {
	refs := make([]noteReference, 0, len(ids))
	for id := range ids {
		if note, ok := byID[id]; ok {
			refs = append(refs, toReference(note, byID))
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Title < refs[j].Title })
	return refs
}

func toReference(note store.Note, byID map[int64]store.Note) noteReference {
	return noteReference{ID: note.ID, Title: displayTitle(note), Path: noteRoutePath(note, byID), Tags: note.Tags}
}

func displayTitle(note store.Note) string {
	title := strings.TrimSpace(note.Title)
	if title == "" {
		return fmt.Sprintf("Untitled#%d", note.ID)
	}
	return title
}

func tagSet(tags []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			out[tag] = struct{}{}
		}
	}
	return out
}

func tokenSet(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range tokenizeText(text) {
		out[token] = struct{}{}
	}
	return out
}

func tokenizeText(text string) []string {
	text = strings.ToLower(plainText(text))
	var tokens []string
	var buf []rune
	flush := func() {
		if len(buf) >= 2 {
			tokens = append(tokens, string(buf))
		}
		buf = buf[:0]
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			buf = append(buf, r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func topKeywords(text string, limit int) []string {
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {}, "http": {},
		"一个": {}, "以及": {}, "可以": {}, "进行": {}, "当前": {}, "内容": {}, "笔记": {},
	}
	counts := map[string]int{}
	for _, token := range tokenizeText(text) {
		if _, ok := stop[token]; ok {
			continue
		}
		if utf8.RuneCountInString(token) < 2 {
			continue
		}
		counts[token]++
	}
	type item struct {
		token string
		count int
	}
	var items []item
	for token, count := range counts {
		items = append(items, item{token, count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].token < items[j].token
	})
	out := make([]string, 0, limit)
	for _, item := range items {
		out = append(out, item.token)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func plainText(markdown string) string {
	text := wikiLinkRE.ReplaceAllString(markdown, "$1")
	text = markdownLinkRE.ReplaceAllString(text, "$2")
	replacers := []string{"#", "", "`", "", "*", "", "_", "", ">", "", "|", " ", "[ ]", "", "[x]", ""}
	text = strings.NewReplacer(replacers...).Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func splitSentences(text string) []string {
	var out []string
	start := 0
	for i, r := range text {
		if strings.ContainsRune("。！？；;.!?", r) {
			part := strings.TrimSpace(text[start : i+len(string(r))])
			if part != "" {
				out = append(out, part)
			}
			start = i + len(string(r))
		}
	}
	if tail := strings.TrimSpace(text[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
}

func firstMeaningfulLine(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return truncateRunes(line, 48)
		}
	}
	return ""
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "..."
}

func sharedStringCount(a, b map[string]struct{}) int {
	count := 0
	for v := range a {
		if _, ok := b[v]; ok {
			count++
		}
	}
	return count
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := sharedStringCount(a, b)
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func recencyScore(t time.Time) float64 {
	days := time.Since(t).Hours() / 24
	if days < 0 {
		days = 0
	}
	return 1 / (1 + days/30)
}

func roundScore(v float64) float64 {
	return math.Round(v*1000) / 1000
}
