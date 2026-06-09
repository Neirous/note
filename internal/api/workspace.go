package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"note/internal/store"
)

type workspaceChartPoint struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type workspaceDashboard struct {
	Stats           map[string]int        `json:"stats"`
	NoteStatusPie   []workspaceChartPoint `json:"note_status_pie"`
	OverviewBars    []workspaceChartPoint `json:"overview_bars"`
	NoteTrend       []workspaceChartPoint `json:"note_trend"`
	UnfinishedNotes []noteReference       `json:"unfinished_notes"`
	RecentActivity  []noteReference       `json:"recent_activity"`
}

type graphNode struct {
	ID           int64    `json:"id"`
	Title        string   `json:"title"`
	Path         string   `json:"path"`
	Tags         []string `json:"tags,omitempty"`
	QualityScore int      `json:"quality_score"`
	UpdatedAt    string   `json:"updated_at"`
}

type graphEdge struct {
	Source int64   `json:"source"`
	Target int64   `json:"target"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
	Reason string  `json:"reason"`
}

type workspaceGraph struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

type qualityHubEvaluationItem struct {
	Note    noteReference `json:"note"`
	Score   int           `json:"score"`
	Issues  []string      `json:"issues"`
	Summary string        `json:"summary"`
	Action  string        `json:"action"`
	Source  string        `json:"source"`
}

type qualityHubEvaluationResponse struct {
	Items  []qualityHubEvaluationItem `json:"items"`
	UsedAI bool                       `json:"used_ai"`
}

type researchSessionResponse struct {
	ID             int64           `json:"id,omitempty"`
	Topic          string          `json:"topic"`
	Summary        string          `json:"summary"`
	RelatedNotes   []noteReference `json:"related_notes"`
	Outline        []string        `json:"outline"`
	Gaps           []string        `json:"gaps"`
	Questions      []string        `json:"questions"`
	SuggestedNotes []string        `json:"suggested_notes"`
	UsedAI         bool            `json:"used_ai"`
	CreatedAt      string          `json:"created_at,omitempty"`
}

type researchSessionHistoryItem struct {
	ID        int64                   `json:"id"`
	Topic     string                  `json:"topic"`
	Result    researchSessionResponse `json:"result"`
	CreatedAt string                  `json:"created_at"`
}

func (s *Server) handleListKnowledgeCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Status:          strings.TrimSpace(r.URL.Query().Get("status")),
		IncludeArchived: parseBool(r.URL.Query().Get("include_archived")),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if cards == nil {
		cards = []store.KnowledgeCard{}
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleDueKnowledgeCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{DueOnly: true})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if cards == nil {
		cards = []store.KnowledgeCard{}
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleGetKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.GetKnowledgeCard(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleCreateKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	var req store.KnowledgeCardInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.CreateKnowledgeCard(ctx, req)
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "front and back are required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) handleUpdateKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req store.KnowledgeCardInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.UpdateKnowledgeCard(ctx, id, req)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "front and back are required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleDeleteKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := s.store.DeleteKnowledgeCard(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleReviewKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Remembered bool   `json:"remembered"`
		Action     string `json:"action"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	remembered := req.Remembered || strings.EqualFold(strings.TrimSpace(req.Action), "remembered")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.ReviewKnowledgeCard(ctx, id, remembered, time.Now())
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleWorkspaceDashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	now := time.Now()
	stats := map[string]int{
		"notes":             len(notes),
		"content_notes":     0,
		"unfinished_notes":  0,
		"completed_notes":   0,
		"today_updated":     0,
		"recent_7d_updated": 0,
	}
	var unfinished []noteReference
	for _, n := range notes {
		if noteHasTag(n, "folder") {
			continue
		}
		stats["content_notes"]++
		if n.Status == "completed" {
			stats["completed_notes"]++
		} else {
			stats["unfinished_notes"]++
			if len(unfinished) < 8 {
				unfinished = append(unfinished, toReference(n, byID))
			}
		}
		if n.UpdatedAt.Local().Format("2006-01-02") == now.Local().Format("2006-01-02") {
			stats["today_updated"]++
		}
		if n.UpdatedAt.After(now.AddDate(0, 0, -7)) {
			stats["recent_7d_updated"]++
		}
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].UpdatedAt.After(notes[j].UpdatedAt) })
	recent := make([]noteReference, 0, 8)
	for _, n := range notes {
		if noteHasTag(n, "folder") {
			continue
		}
		recent = append(recent, toReference(n, byID))
		if len(recent) >= 8 {
			break
		}
	}

	writeJSON(w, http.StatusOK, workspaceDashboard{
		Stats: stats,
		NoteStatusPie: []workspaceChartPoint{
			{Label: "未完成", Value: stats["unfinished_notes"]},
			{Label: "已完成", Value: stats["completed_notes"]},
		},
		OverviewBars:    overviewBars(stats),
		NoteTrend:       noteTrend(notes, now, 14),
		UnfinishedNotes: unfinished,
		RecentActivity:  recent,
	})
}

func (s *Server) handleWorkspaceGraph(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	limit := 80
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	var content []store.Note
	for _, n := range notes {
		if !noteHasTag(n, "folder") {
			content = append(content, n)
		}
	}
	sort.Slice(content, func(i, j int) bool { return content[i].UpdatedAt.After(content[j].UpdatedAt) })
	if len(content) > limit {
		content = content[:limit]
	}
	allowed := map[int64]struct{}{}
	nodes := make([]graphNode, 0, len(content))
	for _, n := range content {
		allowed[n.ID] = struct{}{}
		links := buildLinkReport(n, notes, byID)
		issues, score := assessNoteQuality(n, links)
		_ = issues
		nodes = append(nodes, graphNode{
			ID:           n.ID,
			Title:        displayTitle(n),
			Path:         noteRoutePath(n, byID),
			Tags:         n.Tags,
			QualityScore: score,
			UpdatedAt:    n.UpdatedAt.Format(time.RFC3339),
		})
	}
	edges := buildGraphEdges(content, byID, allowed)
	writeJSON(w, http.StatusOK, workspaceGraph{Nodes: nodes, Edges: edges})
}

func (s *Server) handleWorkspaceQualityEvaluation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var req struct {
		Limit int `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	items := localQualityHubEvaluation(notes, byID, limit)
	if s.rag != nil {
		if aiItems, err := s.aiQualityHubEvaluation(ctx, items, byID, limit); err == nil && len(aiItems) > 0 {
			writeJSON(w, http.StatusOK, qualityHubEvaluationResponse{Items: aiItems, UsedAI: true})
			return
		}
	}
	writeJSON(w, http.StatusOK, qualityHubEvaluationResponse{Items: items, UsedAI: false})
}

func localQualityHubEvaluation(notes []store.Note, byID map[int64]store.Note, limit int) []qualityHubEvaluationItem {
	items := make([]qualityHubEvaluationItem, 0, len(notes))
	for _, note := range notes {
		if noteHasTag(note, "folder") {
			continue
		}
		links := buildLinkReport(note, notes, byID)
		qualityIssues, score := assessNoteQuality(note, links)
		issues := make([]string, 0, len(qualityIssues)+1)
		action := ""
		for _, issue := range qualityIssues {
			if strings.TrimSpace(issue.Message) != "" {
				issues = append(issues, issue.Message)
			}
			if action == "" {
				action = strings.TrimSpace(issue.Suggestion)
			}
		}
		if note.Status != "completed" {
			issues = append([]string{"未完成笔记"}, issues...)
			score -= 10
			if action == "" {
				action = "确认这篇笔记下一步要补什么，完成后把状态标为已完成。"
			}
		}
		if len(issues) == 0 {
			continue
		}
		if score < 0 {
			score = 0
		}
		summary := summarizeMarkdown(note.Markdown, 90)
		if summary == "" {
			summary = "内容较少，适合先补充上下文和结论。"
		}
		items = append(items, qualityHubEvaluationItem{
			Note:    toReference(note, byID),
			Score:   score,
			Issues:  dedupeStrings(issues, 4),
			Summary: summary,
			Action:  action,
			Source:  "local",
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Note.Title < items[j].Note.Title
		}
		return items[i].Score < items[j].Score
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (s *Server) aiQualityHubEvaluation(ctx context.Context, base []qualityHubEvaluationItem, byID map[int64]store.Note, limit int) ([]qualityHubEvaluationItem, error) {
	if s.rag == nil {
		return nil, errors.New("rag not configured")
	}
	if len(base) == 0 {
		return nil, errors.New("no quality candidates")
	}
	var sb strings.Builder
	sb.WriteString("你是知识库体检助手。请评价这些笔记最值得整理的风险点，输出严格 JSON 数组，不要 Markdown 代码块。\n")
	sb.WriteString("数组元素格式：{\"id\":数字,\"score\":0到100整数,\"issues\":[\"具体问题\"],\"summary\":\"一句具体评价\",\"action\":\"下一步动作\"}。\n")
	sb.WriteString("score 越低代表越需要优先整理。issues 和 action 必须针对笔记内容，不要只重复模板词。\n\n")
	sb.WriteString("候选笔记：\n")
	for i, item := range base {
		if i >= limit {
			break
		}
		note, ok := byID[item.Note.ID]
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("ID:%d\n标题:%s\n路径:%s\n状态:%s\n现有标签:%s\n系统初筛:%s\n内容摘录:\n%s\n\n",
			note.ID,
			displayTitle(note),
			noteRoutePath(note, byID),
			note.Status,
			strings.Join(note.Tags, ", "),
			strings.Join(item.Issues, " / "),
			truncateRunes(plainText(note.Markdown), 1200),
		))
	}
	raw, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return nil, err
	}
	parsed := parseQualityHubEvaluationJSON(raw, limit)
	if len(parsed) == 0 {
		return nil, errors.New("empty ai quality evaluation")
	}
	baseByID := map[int64]qualityHubEvaluationItem{}
	for _, item := range base {
		baseByID[item.Note.ID] = item
	}
	out := make([]qualityHubEvaluationItem, 0, len(parsed))
	for _, item := range parsed {
		baseItem, ok := baseByID[item.Note.ID]
		if !ok {
			continue
		}
		item.Note = baseItem.Note
		item.Source = "ai"
		if item.Score < 0 {
			item.Score = 0
		}
		if item.Score > 100 {
			item.Score = 100
		}
		if len(item.Issues) == 0 {
			item.Issues = baseItem.Issues
		}
		if strings.TrimSpace(item.Summary) == "" {
			item.Summary = baseItem.Summary
		}
		if strings.TrimSpace(item.Action) == "" {
			item.Action = baseItem.Action
		}
		item.Issues = dedupeStrings(item.Issues, 4)
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Note.Title < out[j].Note.Title
		}
		return out[i].Score < out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func parseQualityHubEvaluationJSON(raw string, limit int) []qualityHubEvaluationItem {
	text := strings.TrimSpace(unwrapMarkdownFence(raw))
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		text = text[start : end+1]
	}
	var payload []struct {
		ID      int64    `json:"id"`
		Score   int      `json:"score"`
		Issues  []string `json:"issues"`
		Summary string   `json:"summary"`
		Action  string   `json:"action"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	out := make([]qualityHubEvaluationItem, 0, len(payload))
	for _, item := range payload {
		if item.ID <= 0 {
			continue
		}
		out = append(out, qualityHubEvaluationItem{
			Note:    noteReference{ID: item.ID},
			Score:   item.Score,
			Issues:  item.Issues,
			Summary: strings.TrimSpace(item.Summary),
			Action:  strings.TrimSpace(item.Action),
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func dedupeStrings(values []string, limit int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Server) handleResearchSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string  `json:"topic"`
		NoteIDs []int64 `json:"note_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		writeErrMsg(w, http.StatusBadRequest, "topic is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	notes, byID, err := s.activeNotesWithIndex(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	related := researchRelatedNotes(topic, req.NoteIDs, notes)
	resp := fallbackResearchSession(topic, related, byID)
	if ai, err := s.aiResearchSession(ctx, topic, related, byID); err == nil && strings.TrimSpace(ai.Summary) != "" {
		resp = ai
		resp.UsedAI = true
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	session, err := s.store.CreateResearchSession(ctx, topic, raw)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.ID = session.ID
	resp.CreatedAt = session.CreatedAt.Format(time.RFC3339)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleListResearchSessions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	sessions, err := s.store.ListResearchSessions(ctx, 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]researchSessionHistoryItem, 0, len(sessions))
	for _, session := range sessions {
		item, err := researchHistoryItemFromStore(session)
		if err == nil {
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteResearchSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := s.store.DeleteResearchSession(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "research session not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func researchHistoryItemFromStore(session store.ResearchSession) (researchSessionHistoryItem, error) {
	var result researchSessionResponse
	if err := json.Unmarshal([]byte(session.Result), &result); err != nil {
		return researchSessionHistoryItem{}, err
	}
	result.ID = session.ID
	result.CreatedAt = session.CreatedAt.Format(time.RFC3339)
	if strings.TrimSpace(result.Topic) == "" {
		result.Topic = session.Topic
	}
	return researchSessionHistoryItem{
		ID:        session.ID,
		Topic:     session.Topic,
		Result:    result,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}, nil
}

func overviewBars(stats map[string]int) []workspaceChartPoint {
	return []workspaceChartPoint{
		{Label: "今日更新", Value: stats["today_updated"]},
		{Label: "近 7 天更新", Value: stats["recent_7d_updated"]},
		{Label: "未完成笔记", Value: stats["unfinished_notes"]},
		{Label: "已完成笔记", Value: stats["completed_notes"]},
	}
}

func noteTrend(notes []store.Note, now time.Time, days int) []workspaceChartPoint {
	counts := dayBuckets(now, days)
	for _, n := range notes {
		key := n.UpdatedAt.Local().Format("01-02")
		if _, ok := counts[key]; ok {
			counts[key]++
		}
	}
	return bucketPoints(now, days, counts)
}

func cardReviewTrend(cards []store.KnowledgeCard, now time.Time, days int) []workspaceChartPoint {
	counts := dayBuckets(now, days)
	for _, card := range cards {
		if card.LastReviewedAt == nil {
			continue
		}
		key := card.LastReviewedAt.Local().Format("01-02")
		if _, ok := counts[key]; ok {
			counts[key]++
		}
	}
	return bucketPoints(now, days, counts)
}

func dayBuckets(now time.Time, days int) map[string]int {
	out := make(map[string]int, days)
	for i := days - 1; i >= 0; i-- {
		out[now.AddDate(0, 0, -i).Local().Format("01-02")] = 0
	}
	return out
}

func bucketPoints(now time.Time, days int, counts map[string]int) []workspaceChartPoint {
	out := make([]workspaceChartPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		key := now.AddDate(0, 0, -i).Local().Format("01-02")
		out = append(out, workspaceChartPoint{Label: key, Value: counts[key]})
	}
	return out
}

func buildGraphEdges(notes []store.Note, byID map[int64]store.Note, allowed map[int64]struct{}) []graphEdge {
	edgesByKey := map[string]graphEdge{}
	add := func(source, target int64, typ string, weight float64, reason string) {
		if source == target {
			return
		}
		if _, ok := allowed[source]; !ok {
			return
		}
		if _, ok := allowed[target]; !ok {
			return
		}
		key := fmt.Sprintf("%d:%d:%s", source, target, typ)
		if typ != "link" && source > target {
			key = fmt.Sprintf("%d:%d:%s", target, source, typ)
		}
		if prev, ok := edgesByKey[key]; ok && prev.Weight >= weight {
			return
		}
		edgesByKey[key] = graphEdge{Source: source, Target: target, Type: typ, Weight: roundScore(weight), Reason: reason}
	}
	for _, n := range notes {
		for target := range linkedNoteIDs(n, byID) {
			add(n.ID, target, "link", 1, "内部链接")
		}
	}
	for i := range notes {
		for j := i + 1; j < len(notes); j++ {
			a, b := notes[i], notes[j]
			shared := sharedStringCount(tagSet(a.Tags), tagSet(b.Tags))
			if shared > 0 {
				add(a.ID, b.ID, "tag", math.Min(0.3+float64(shared)*0.2, 0.9), "共享标签")
			}
			score := jaccard(tokenSet(a.Title+" "+a.Markdown), tokenSet(b.Title+" "+b.Markdown))
			if score >= 0.18 {
				add(a.ID, b.ID, "similar", score, "内容相似")
			}
		}
	}
	edges := make([]graphEdge, 0, len(edgesByKey))
	for _, edge := range edgesByKey {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Weight != edges[j].Weight {
			return edges[i].Weight > edges[j].Weight
		}
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
	if len(edges) > 240 {
		edges = edges[:240]
	}
	return edges
}

func researchRelatedNotes(topic string, noteIDs []int64, notes []store.Note) []store.Note {
	selected := make(map[int64]struct{})
	for _, id := range noteIDs {
		if id > 0 {
			selected[id] = struct{}{}
		}
	}
	type scored struct {
		n     store.Note
		score float64
	}
	topic = strings.TrimSpace(topic)
	topicLower := strings.ToLower(topic)
	topicTokens := tokenSet(topic)
	searchTokens := researchSearchTokens(topic)
	var ranked []scored
	for _, n := range notes {
		if noteHasTag(n, "folder") {
			continue
		}
		titleLower := strings.ToLower(n.Title)
		bodyLower := strings.ToLower(n.Markdown)
		tagsLower := strings.ToLower(strings.Join(n.Tags, " "))
		haystack := strings.Join([]string{titleLower, bodyLower, tagsLower}, " ")
		score := jaccard(topicTokens, tokenSet(n.Title+" "+strings.Join(n.Tags, " ")+" "+n.Markdown)) * 1.5
		if _, ok := selected[n.ID]; ok {
			score += 2
		}
		if topicLower != "" {
			if strings.Contains(titleLower, topicLower) {
				score += 1.2
			} else if strings.Contains(haystack, topicLower) {
				score += 0.8
			}
		}
		for _, token := range searchTokens {
			if strings.Contains(titleLower, token) {
				score += 0.8
			}
			if noteHasTag(n, token) || strings.Contains(tagsLower, token) {
				score += 1
			}
			if strings.Contains(bodyLower, token) {
				score += 0.25
			}
		}
		if score > 0 {
			ranked = append(ranked, scored{n: n, score: score})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].n.UpdatedAt.After(ranked[j].n.UpdatedAt)
	})
	out := make([]store.Note, 0, 8)
	for _, item := range ranked {
		out = append(out, item.n)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

var researchSearchTokenRE = regexp.MustCompile(`[a-z0-9_+\-.]+|[\p{Han}]{2,}`)

func researchSearchTokens(topic string) []string {
	seen := map[string]struct{}{}
	add := func(token string) {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			return
		}
		seen[token] = struct{}{}
	}
	for token := range tokenSet(topic) {
		add(token)
	}
	for _, token := range researchSearchTokenRE.FindAllString(strings.ToLower(topic), -1) {
		add(token)
	}
	out := make([]string, 0, len(seen))
	for token := range seen {
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func fallbackResearchSession(topic string, notes []store.Note, byID map[int64]store.Note) researchSessionResponse {
	refs := make([]noteReference, 0, len(notes))
	var text strings.Builder
	for _, n := range notes {
		refs = append(refs, toReference(n, byID))
		text.WriteString(n.Title)
		text.WriteByte(' ')
		text.WriteString(n.Markdown)
		text.WriteByte(' ')
	}
	keywords := topKeywords(topic+" "+text.String(), 8)
	if len(keywords) == 0 {
		keywords = []string{topic}
	}
	return researchSessionResponse{
		Topic:        topic,
		Summary:      fmt.Sprintf("围绕“%s”，当前知识库已关联 %d 篇笔记。建议先梳理已有材料，再补齐概念、案例和实现路径。", topic, len(notes)),
		RelatedNotes: refs,
		Outline: []string{
			"研究背景与核心问题",
			"已有笔记材料归纳",
			"关键概念与技术路线",
			"实践场景与验证方式",
			"阶段结论与后续计划",
		},
		Gaps: []string{
			"补充更明确的定义和边界",
			"加入可验证的案例或数据",
			"整理与现有笔记之间的链接关系",
		},
		Questions: []string{
			fmt.Sprintf("%s 的核心概念是什么？", topic),
			fmt.Sprintf("%s 可以解决哪些具体问题？", topic),
			fmt.Sprintf("如何验证 %s 的效果？", topic),
		},
		SuggestedNotes: []string{
			topic + " 研究提纲",
			topic + " 关键问题清单",
			topic + " 实践案例整理",
			"与 " + strings.Join(keywords[:minInt(len(keywords), 3)], "、") + " 相关的资料卡",
		},
		UsedAI: false,
	}
}

func (s *Server) aiResearchSession(ctx context.Context, topic string, notes []store.Note, byID map[int64]store.Note) (researchSessionResponse, error) {
	if s.rag == nil {
		return researchSessionResponse{}, errors.New("rag not configured")
	}
	var sb strings.Builder
	sb.WriteString("请基于用户笔记生成主题研究室结果，输出严格 JSON，不要 Markdown 代码块。\n")
	sb.WriteString(`JSON 字段：summary(string), outline(string[]), gaps(string[]), questions(string[]), suggested_notes(string[]).`)
	sb.WriteString("\n主题：")
	sb.WriteString(topic)
	sb.WriteString("\n相关笔记：\n")
	for i, n := range notes {
		if i >= 6 {
			break
		}
		sb.WriteString(fmt.Sprintf("笔记%d：%s\n%s\n\n", i+1, n.Title, summarizeMarkdown(n.Markdown, 600)))
	}
	raw, err := s.rag.Generate(ctx, sb.String())
	if err != nil {
		return researchSessionResponse{}, err
	}
	clean := strings.TrimSpace(strings.Trim(raw, "`"))
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	var parsed struct {
		Summary        string   `json:"summary"`
		Outline        []string `json:"outline"`
		Gaps           []string `json:"gaps"`
		Questions      []string `json:"questions"`
		SuggestedNotes []string `json:"suggested_notes"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return researchSessionResponse{}, err
	}
	refs := make([]noteReference, 0, len(notes))
	for _, n := range notes {
		refs = append(refs, toReference(n, byID))
	}
	return researchSessionResponse{
		Topic:          topic,
		Summary:        parsed.Summary,
		RelatedNotes:   refs,
		Outline:        parsed.Outline,
		Gaps:           parsed.Gaps,
		Questions:      parsed.Questions,
		SuggestedNotes: parsed.SuggestedNotes,
		UsedAI:         true,
	}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
