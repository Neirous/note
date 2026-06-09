package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"note/internal/store"
)

func createTestNote(t *testing.T, srv *Server, title, markdown string, tags []string, parentID *int64) store.Note {
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

func TestIntelligenceInsightsRecommendationsAndLinks(t *testing.T) {
	srv := newTestServer(t)

	folder := createTestNote(t, srv, "Backend", "# Backend", []string{"folder"}, nil)
	apiGuide := createTestNote(t, srv, "API Guide", "# API Guide\n\nHTTP handler middleware error response", []string{"go", "api"}, &folder.ID)
	dbGuide := createTestNote(t, srv, "SQLite Guide", "# SQLite Guide\n\nSQLite database migration and query", []string{"go", "database"}, &folder.ID)
	draft := createTestNote(t, srv, "Backend Draft", "# Backend Draft\n\nAPI Guide talks about HTTP handler.\n\nSee [[API Guide]].\n\n- [ ] finish handler tests", []string{"go"}, nil)
	_ = dbGuide

	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(draft.ID)+"/insights", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var insights noteInsights
	if err := json.Unmarshal(rec.Body.Bytes(), &insights); err != nil {
		t.Fatal(err)
	}
	if insights.Summary == "" {
		t.Fatal("expected summary")
	}
	if len(insights.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if len(insights.Links.Outgoing) != 1 || insights.Links.Outgoing[0].ID != apiGuide.ID {
		t.Fatalf("expected outgoing API Guide link, got %+v", insights.Links.Outgoing)
	}
	if insights.QualityScore <= 0 {
		t.Fatalf("expected quality score, got %d", insights.QualityScore)
	}
	if len(insights.Flashcards) != 0 {
		t.Fatalf("expected no passive review questions, got %+v", insights.Flashcards)
	}
}

func TestSuggestTagsRejectsChineseSentenceFragments(t *testing.T) {
	note := store.Note{
		Title: "行为金融与投资纪律",
		Markdown: `# 行为金融与投资纪律

投资者常高估自己识别拐点的能力，低估亏损厌恶、从众、近期偏差和过度交易的影响。
一个可靠的方法通常包含三步：先把对象边界写清楚，再定义判断标准，最后保留复盘证据。
这篇笔记也会提到 RAG 检索，但它不是投资主题的标签。`,
		Tags: []string{"behavior", "finance"},
	}

	got := suggestTags(note, nil, 6)
	for _, tag := range got {
		if strings.Contains(tag, "一个可靠的方法") || strings.Contains(tag, "投资者常高估") {
			t.Fatalf("suggested sentence fragment as tag: %+v", got)
		}
		if containsCJK(tag) && len([]rune(tag)) > 8 {
			t.Fatalf("suggested overlong Chinese tag %q in %+v", tag, got)
		}
	}
	if !containsString(got, "rag") {
		t.Fatalf("expected concise latin keyword tag rag, got %+v", got)
	}
}

func TestNoteInsightsUsesAIWhenAvailable(t *testing.T) {
	gen := &spyGenerator{
		response: `{
			"summary":"投资纪律笔记聚焦行为偏差如何影响交易决策。",
			"outline":["识别亏损厌恶与从众影响","用交易前假设约束操作","复盘证据和风险边界"],
			"keywords":["投资纪律","行为偏差","复盘"],
			"suggested_tags":["投资纪律","finance","一个可靠的方法通常包含三步"],
			"quality_score":87,
			"quality_issues":[{"type":"evidence","severity":"medium","message":"缺少可复盘的真实案例。","suggestion":"补充一次具体交易前后的判断证据。"}]
		}`,
	}
	srv := newTestServerWithGenerator(t, gen)
	note := createTestNote(t, srv, "行为金融与投资纪律", "# 行为金融与投资纪律\n\n投资者常高估自己识别拐点的能力，需要用规则约束交易。", []string{"finance"}, nil)
	createTestNote(t, srv, "交易复盘模板", "# 交易复盘模板\n\n记录投资纪律、行为偏差和交易前假设。", []string{"finance", "review"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/insights", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 insights, got %d body=%s", rec.Code, rec.Body.String())
	}
	var insights noteInsights
	if err := json.Unmarshal(rec.Body.Bytes(), &insights); err != nil {
		t.Fatal(err)
	}
	if !insights.UsedAI {
		t.Fatalf("expected AI insights, got %+v", insights)
	}
	if insights.Summary != "投资纪律笔记聚焦行为偏差如何影响交易决策。" || insights.QualityScore != 87 {
		t.Fatalf("expected AI summary and score, got %+v", insights)
	}
	if !containsString(insights.SuggestedTags, "投资纪律") {
		t.Fatalf("expected AI concise tag, got %+v", insights.SuggestedTags)
	}
	if containsString(insights.SuggestedTags, "finance") || containsString(insights.SuggestedTags, "一个可靠的方法通常包含三步") {
		t.Fatalf("expected existing and sentence tags to be filtered, got %+v", insights.SuggestedTags)
	}
	if len(insights.Recommendations) == 0 {
		t.Fatalf("expected local relationship tasks to remain populated, got %+v", insights.Recommendations)
	}
}

func TestNoteInsightsCachedReadDoesNotRegenerate(t *testing.T) {
	gen := &spyGenerator{
		response: `{
			"summary":"缓存后的洞察摘要。",
			"outline":["缓存读取"],
			"keywords":["缓存"],
			"suggested_tags":["缓存"],
			"quality_score":81,
			"quality_issues":[]
		}`,
	}
	srv := newTestServerWithGenerator(t, gen)
	note := createTestNote(t, srv, "缓存洞察", "# 缓存洞察\n\n用户主动刷新后应该保存结果。", nil, nil)

	cachedBefore := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/insights?cached=1", nil)
	cachedBeforeRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(cachedBeforeRec, cachedBefore)
	if cachedBeforeRec.Code != http.StatusNotFound {
		t.Fatalf("expected cached miss 404, got %d body=%s", cachedBeforeRec.Code, cachedBeforeRec.Body.String())
	}

	refreshReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/insights", nil)
	refreshRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(refreshRec, refreshReq)
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected refresh 200, got %d body=%s", refreshRec.Code, refreshRec.Body.String())
	}
	if gen.calls != 1 {
		t.Fatalf("expected one AI generation after refresh, got %d", gen.calls)
	}

	cachedReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/insights?cached=1", nil)
	cachedRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(cachedRec, cachedReq)
	if cachedRec.Code != http.StatusOK {
		t.Fatalf("expected cached 200, got %d body=%s", cachedRec.Code, cachedRec.Body.String())
	}
	if gen.calls != 1 {
		t.Fatalf("cached read regenerated insights; calls=%d", gen.calls)
	}
	var cached noteInsights
	if err := json.Unmarshal(cachedRec.Body.Bytes(), &cached); err != nil {
		t.Fatal(err)
	}
	if cached.Summary != "缓存后的洞察摘要。" || cached.QualityScore != 81 {
		t.Fatalf("expected cached insights, got %+v", cached)
	}
}

func TestSuggestTagsEndpointUsesAIWhenAvailable(t *testing.T) {
	gen := &spyGenerator{response: `["投资纪律","finance","一个可靠的方法通常包含三步"]`}
	srv := newTestServerWithGenerator(t, gen)
	note := createTestNote(t, srv, "行为金融与投资纪律", "# 行为金融与投资纪律\n\n投资者需要识别亏损厌恶和过度交易。", []string{"finance"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/suggest-tags", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 suggest tags, got %d body=%s", rec.Code, rec.Body.String())
	}
	var tags []string
	if err := json.Unmarshal(rec.Body.Bytes(), &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "投资纪律" {
		t.Fatalf("expected sanitized AI tags, got %+v", tags)
	}
}

func TestReviewQuestionFlow(t *testing.T) {
	srv := newTestServer(t)

	note := createTestNote(t, srv, "Reviewable", "# Reviewable\n\n## 核心概念\n\n复习问题需要由用户主动创建。", []string{"study"}, nil)

	createBody, _ := json.Marshal(map[string]any{
		"question": "为什么复习问题要主动创建？",
		"answer":   "避免 AI 被动写死问题，用户确认后再沉淀。",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+itoa(note.ID)+"/review-questions", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 create review question, got %d body=%s", createRec.Code, createRec.Body.String())
	}
	var created store.ReviewQuestion
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Source != "manual" {
		t.Fatalf("expected manual source, got %+v", created)
	}

	insightsReq := httptest.NewRequest(http.MethodGet, "/api/notes/"+itoa(note.ID)+"/insights", nil)
	insightsRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(insightsRec, insightsReq)
	if insightsRec.Code != http.StatusOK {
		t.Fatalf("expected 200 insights, got %d body=%s", insightsRec.Code, insightsRec.Body.String())
	}
	var insights noteInsights
	if err := json.Unmarshal(insightsRec.Body.Bytes(), &insights); err != nil {
		t.Fatal(err)
	}
	if len(insights.Flashcards) != 1 || insights.Flashcards[0].ID != created.ID {
		t.Fatalf("expected saved review question in insights, got %+v", insights.Flashcards)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"question": "AI 生成的问题如何处理？",
		"answer":   "用户可以继续编辑。",
	})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/notes/"+itoa(note.ID)+"/review-questions/"+itoa(created.ID), bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200 update, got %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	genReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+itoa(note.ID)+"/review-questions/generate", strings.NewReader(`{"count":2}`))
	genReq.Header.Set("Content-Type", "application/json")
	genRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(genRec, genReq)
	if genRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 generate, got %d body=%s", genRec.Code, genRec.Body.String())
	}
	var generated struct {
		Created []store.ReviewQuestion `json:"created"`
		Items   []store.ReviewQuestion `json:"items"`
	}
	if err := json.Unmarshal(genRec.Body.Bytes(), &generated); err != nil {
		t.Fatal(err)
	}
	if len(generated.Created) == 0 || generated.Created[0].Source != "ai" {
		t.Fatalf("expected generated ai questions, got %+v", generated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/notes/"+itoa(note.ID)+"/review-questions/"+itoa(created.ID), nil)
	deleteRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete, got %d body=%s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestIntelligenceReviewTasksTemplatesImportAndRecommend(t *testing.T) {
	srv := newTestServer(t)

	project := createTestNote(t, srv, "Project Log", "# Project Log\n\n- [ ] ship recommendation panel", []string{"project"}, nil)
	createTestNote(t, srv, "Reading", "# Reading\n\n- [x] collect notes", []string{"reading"}, nil)

	tasksReq := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	tasksRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(tasksRec, tasksReq)
	if tasksRec.Code != http.StatusOK {
		t.Fatalf("expected 200 tasks, got %d body=%s", tasksRec.Code, tasksRec.Body.String())
	}
	var tasks []taskItem
	if err := json.Unmarshal(tasksRec.Body.Bytes(), &tasks); err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Checked {
		t.Fatalf("expected unfinished task first, got %+v", tasks)
	}

	templatesReq := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	templatesRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(templatesRec, templatesReq)
	if templatesRec.Code != http.StatusOK {
		t.Fatalf("expected 200 templates, got %d", templatesRec.Code)
	}
	if !strings.Contains(templatesRec.Body.String(), "study") {
		t.Fatalf("expected study template, got %s", templatesRec.Body.String())
	}

	createTplBody, _ := json.Marshal(map[string]any{"title": "Study From Template"})
	tplReq := httptest.NewRequest(http.MethodPost, "/api/templates/study/notes", bytes.NewReader(createTplBody))
	tplReq.Header.Set("Content-Type", "application/json")
	tplRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(tplRec, tplReq)
	if tplRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 template note, got %d body=%s", tplRec.Code, tplRec.Body.String())
	}

	importBody, _ := json.Marshal(map[string]any{
		"title":    "Imported",
		"markdown": "# Imported\n\nplain imported content",
		"tags":     []string{"import"},
	})
	importReq := httptest.NewRequest(http.MethodPost, "/api/import", bytes.NewReader(importBody))
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 import, got %d body=%s", importRec.Code, importRec.Body.String())
	}

	reviewReq := httptest.NewRequest(http.MethodGet, "/api/review", nil)
	reviewRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(reviewRec, reviewReq)
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 review, got %d body=%s", reviewRec.Code, reviewRec.Body.String())
	}
	var review reviewReport
	if err := json.Unmarshal(reviewRec.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	if len(review.RecommendedNext) == 0 {
		t.Fatal("expected recommended notes")
	}

	recommendBody, _ := json.Marshal(map[string]any{
		"topic":    "recommendation panel",
		"note_ids": []int64{project.ID},
	})
	recommendReq := httptest.NewRequest(http.MethodPost, "/api/recommend", bytes.NewReader(recommendBody))
	recommendReq.Header.Set("Content-Type", "application/json")
	recommendRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(recommendRec, recommendReq)
	if recommendRec.Code != http.StatusOK {
		t.Fatalf("expected 200 recommend, got %d body=%s", recommendRec.Code, recommendRec.Body.String())
	}
	var aiRec aiRecommendationResponse
	if err := json.Unmarshal(recommendRec.Body.Bytes(), &aiRec); err != nil {
		t.Fatal(err)
	}
	if aiRec.Summary == "" || len(aiRec.References) != 1 {
		t.Fatalf("expected summary and selected reference, got %+v", aiRec)
	}
	if aiRec.ID == 0 || aiRec.Topic != "recommendation panel" || aiRec.CreatedAt == "" {
		t.Fatalf("expected saved recommendation metadata, got %+v", aiRec)
	}
	recommendHistoryReq := httptest.NewRequest(http.MethodGet, "/api/recommend/sessions", nil)
	recommendHistoryRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(recommendHistoryRec, recommendHistoryReq)
	if recommendHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 recommendation history, got %d body=%s", recommendHistoryRec.Code, recommendHistoryRec.Body.String())
	}
	var recommendHistory []recommendationSessionHistoryItem
	if err := json.Unmarshal(recommendHistoryRec.Body.Bytes(), &recommendHistory); err != nil {
		t.Fatal(err)
	}
	if len(recommendHistory) != 1 || recommendHistory[0].ID != aiRec.ID || recommendHistory[0].Result.Topic != "recommendation panel" {
		t.Fatalf("expected saved recommendation history item, got %+v", recommendHistory)
	}
	deleteRecommendHistoryReq := httptest.NewRequest(http.MethodDelete, "/api/recommend/sessions/"+itoa(aiRec.ID), nil)
	deleteRecommendHistoryRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(deleteRecommendHistoryRec, deleteRecommendHistoryReq)
	if deleteRecommendHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete recommendation history, got %d body=%s", deleteRecommendHistoryRec.Code, deleteRecommendHistoryRec.Body.String())
	}

	reportFolder := createTestNote(t, srv, "Reports", "# Reports", []string{"folder"}, nil)
	weeklyBody, _ := json.Marshal(map[string]any{
		"title":     "Weekly Report",
		"parent_id": reportFolder.ID,
	})
	weeklyReq := httptest.NewRequest(http.MethodPost, "/api/writing/weekly-report", bytes.NewReader(weeklyBody))
	weeklyReq.Header.Set("Content-Type", "application/json")
	weeklyRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(weeklyRec, weeklyReq)
	if weeklyRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 weekly report, got %d body=%s", weeklyRec.Code, weeklyRec.Body.String())
	}
	var weekly weeklyReportResponse
	if err := json.Unmarshal(weeklyRec.Body.Bytes(), &weekly); err != nil {
		t.Fatal(err)
	}
	if weekly.Note.Title != "Weekly Report" || weekly.Note.ParentID == nil || *weekly.Note.ParentID != reportFolder.ID {
		t.Fatalf("expected weekly report in folder, got %+v", weekly.Note)
	}
	if !strings.Contains(weekly.Note.Markdown, "本周学习大纲") || !strings.Contains(weekly.Note.Markdown, "下周学习建议") || !strings.Contains(weekly.Note.Markdown, "资源推荐") {
		t.Fatalf("expected structured weekly report markdown, got %s", weekly.Note.Markdown)
	}
	if strings.Contains(weekly.Note.Markdown, "note://") || !strings.Contains(weekly.Note.Markdown, "[[") {
		t.Fatalf("expected weekly report to use readable wiki links, got %s", weekly.Note.Markdown)
	}
}

func TestWorkspaceCardsGraphAndResearch(t *testing.T) {
	srv := newTestServer(t)

	n1 := createTestNote(t, srv, "RAG System", "# RAG System\n\n检索增强生成依赖向量搜索和上下文问答。See [[Vector Search]].", []string{"rag", "ai"}, nil)
	n2 := createTestNote(t, srv, "Vector Search", "# Vector Search\n\n向量搜索用于找到相似文本，是 RAG 的检索基础。", []string{"rag", "search"}, nil)

	statusBody, _ := json.Marshal(map[string]string{"status": "completed"})
	statusReq := httptest.NewRequest(http.MethodPatch, "/api/notes/"+itoa(n1.ID)+"/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")
	statusRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 note status, got %d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var updated store.Note
	if err := json.Unmarshal(statusRec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != "completed" {
		t.Fatalf("expected completed status, got %+v", updated)
	}

	cardBody, _ := json.Marshal(map[string]any{
		"front": "RAG 的检索阶段做什么？",
		"back":  "从知识库中找到相关上下文。",
		"tags":  []string{"rag"},
	})
	cardReq := httptest.NewRequest(http.MethodPost, "/api/cards", bytes.NewReader(cardBody))
	cardReq.Header.Set("Content-Type", "application/json")
	cardRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(cardRec, cardReq)
	if cardRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 card, got %d body=%s", cardRec.Code, cardRec.Body.String())
	}
	var card store.KnowledgeCard
	if err := json.Unmarshal(cardRec.Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}

	reviewReq := httptest.NewRequest(http.MethodPost, "/api/cards/"+itoa(card.ID)+"/review", strings.NewReader(`{"remembered":true}`))
	reviewReq.Header.Set("Content-Type", "application/json")
	reviewRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(reviewRec, reviewReq)
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 card review, got %d body=%s", reviewRec.Code, reviewRec.Body.String())
	}
	var reviewed store.KnowledgeCard
	if err := json.Unmarshal(reviewRec.Body.Bytes(), &reviewed); err != nil {
		t.Fatal(err)
	}
	if reviewed.ReviewStage != 1 || reviewed.NextReviewAt == nil {
		t.Fatalf("expected reviewed stage 1, got %+v", reviewed)
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/api/workspace/dashboard", nil)
	dashboardRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("expected 200 dashboard, got %d body=%s", dashboardRec.Code, dashboardRec.Body.String())
	}
	var dashboard workspaceDashboard
	if err := json.Unmarshal(dashboardRec.Body.Bytes(), &dashboard); err != nil {
		t.Fatal(err)
	}
	if len(dashboard.NoteStatusPie) == 0 || len(dashboard.OverviewBars) == 0 {
		t.Fatalf("expected chart data, got %+v", dashboard)
	}

	graphReq := httptest.NewRequest(http.MethodGet, "/api/workspace/graph", nil)
	graphRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(graphRec, graphReq)
	if graphRec.Code != http.StatusOK {
		t.Fatalf("expected 200 graph, got %d body=%s", graphRec.Code, graphRec.Body.String())
	}
	var graph workspaceGraph
	if err := json.Unmarshal(graphRec.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) < 2 || len(graph.Edges) == 0 {
		t.Fatalf("expected graph nodes and edges for notes %d/%d, got %+v", n1.ID, n2.ID, graph)
	}

	qualityReq := httptest.NewRequest(http.MethodPost, "/api/workspace/quality-evaluation", strings.NewReader(`{"limit":5}`))
	qualityReq.Header.Set("Content-Type", "application/json")
	qualityRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(qualityRec, qualityReq)
	if qualityRec.Code != http.StatusOK {
		t.Fatalf("expected 200 quality evaluation, got %d body=%s", qualityRec.Code, qualityRec.Body.String())
	}
	var quality qualityHubEvaluationResponse
	if err := json.Unmarshal(qualityRec.Body.Bytes(), &quality); err != nil {
		t.Fatal(err)
	}
	if len(quality.Items) == 0 || quality.Items[0].Note.ID == 0 || len(quality.Items[0].Issues) == 0 {
		t.Fatalf("expected quality evaluation items, got %+v", quality)
	}

	researchBody, _ := json.Marshal(map[string]string{"topic": "RAG"})
	researchReq := httptest.NewRequest(http.MethodPost, "/api/research/session", bytes.NewReader(researchBody))
	researchReq.Header.Set("Content-Type", "application/json")
	researchRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(researchRec, researchReq)
	if researchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 research, got %d body=%s", researchRec.Code, researchRec.Body.String())
	}
	var research researchSessionResponse
	if err := json.Unmarshal(researchRec.Body.Bytes(), &research); err != nil {
		t.Fatal(err)
	}
	if research.Summary == "" || len(research.Questions) == 0 || len(research.RelatedNotes) == 0 {
		t.Fatalf("expected useful research response, got %+v", research)
	}
	if research.ID == 0 || research.CreatedAt == "" {
		t.Fatalf("expected saved research metadata, got %+v", research)
	}

	historyReq := httptest.NewRequest(http.MethodGet, "/api/research/sessions", nil)
	historyRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(historyRec, historyReq)
	if historyRec.Code != http.StatusOK {
		t.Fatalf("expected 200 research history, got %d body=%s", historyRec.Code, historyRec.Body.String())
	}
	var history []researchSessionHistoryItem
	if err := json.Unmarshal(historyRec.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != research.ID || history[0].Result.Topic != "RAG" {
		t.Fatalf("expected saved research history item, got %+v", history)
	}

	deleteHistoryReq := httptest.NewRequest(http.MethodDelete, "/api/research/sessions/"+itoa(research.ID), nil)
	deleteHistoryRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(deleteHistoryRec, deleteHistoryReq)
	if deleteHistoryRec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete research history, got %d body=%s", deleteHistoryRec.Code, deleteHistoryRec.Body.String())
	}
}

func TestWeeklyReportNormalizesNumberedNoteReferences(t *testing.T) {
	gen := &spyGenerator{
		response: "# 本周学习周报\n\n## 本周学习大纲\n\n- 结合笔记[1][2]复盘本周重点。\n\n## 重点理解\n\n- 按笔记 [1] 梳理关键概念，形成可回看的总结。\n\n## 下周学习建议\n\n- 继续补充示例并建立行动清单。\n\n## 资源推荐\n\n- 先阅读本周笔记再扩展官方资料。",
	}
	srv := newTestServerWithGenerator(t, gen)

	first := createTestNote(t, srv, "Alpha Note", "# Alpha Note\n\nalpha content", []string{"weekly"}, nil)
	second := createTestNote(t, srv, "Beta Note", "# Beta Note\n\nbeta content", []string{"weekly"}, nil)

	weeklyBody, _ := json.Marshal(map[string]any{"title": "Linked Weekly Report"})
	weeklyReq := httptest.NewRequest(http.MethodPost, "/api/writing/weekly-report", bytes.NewReader(weeklyBody))
	weeklyReq.Header.Set("Content-Type", "application/json")
	weeklyRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(weeklyRec, weeklyReq)
	if weeklyRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 weekly report, got %d body=%s", weeklyRec.Code, weeklyRec.Body.String())
	}
	var weekly weeklyReportResponse
	if err := json.Unmarshal(weeklyRec.Body.Bytes(), &weekly); err != nil {
		t.Fatal(err)
	}
	if weeklyNumberedNoteRefRE.MatchString(weekly.Note.Markdown) {
		t.Fatalf("expected numbered note references to be normalized, got %s", weekly.Note.Markdown)
	}
	if !strings.Contains(weekly.Note.Markdown, "[["+first.Title+"]]") || !strings.Contains(weekly.Note.Markdown, "[["+second.Title+"]]") {
		t.Fatalf("expected readable links to both source notes, got %s", weekly.Note.Markdown)
	}
}

func TestWeeklyReportUsesSelectedNotesAndLocalFiles(t *testing.T) {
	gen := &spyGenerator{
		response: "# 本周学习周报\n\n## 本周学习大纲\n\n- 基于选择的笔记和本机文件整理学习进展。\n\n## 重点理解\n\n- 本机文件补充了本周项目复盘材料。\n\n## 下周学习建议\n\n- 继续把复盘内容拆成可执行清单。\n\n## 资源推荐\n\n- 优先阅读本周选定材料对应的官方文档。",
	}
	srv := newTestServerWithGenerator(t, gen)

	selected := createTestNote(t, srv, "Selected Note", "# Selected Note\n\nchosen content", []string{"weekly"}, nil)
	unselected := createTestNote(t, srv, "Unselected Note", "# Unselected Note\n\nshould not be used", []string{"weekly"}, nil)

	weeklyBody, _ := json.Marshal(map[string]any{
		"title":    "Custom Weekly Report",
		"note_ids": []int64{selected.ID},
		"file_sources": []map[string]string{
			{"name": "local-review.md", "content": "# Local Review\n\n本机文件复盘内容"},
		},
	})
	weeklyReq := httptest.NewRequest(http.MethodPost, "/api/writing/weekly-report", bytes.NewReader(weeklyBody))
	weeklyReq.Header.Set("Content-Type", "application/json")
	weeklyRec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(weeklyRec, weeklyReq)
	if weeklyRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 weekly report, got %d body=%s", weeklyRec.Code, weeklyRec.Body.String())
	}
	var weekly weeklyReportResponse
	if err := json.Unmarshal(weeklyRec.Body.Bytes(), &weekly); err != nil {
		t.Fatal(err)
	}
	if len(weekly.Sources) != 1 || weekly.Sources[0].ID != selected.ID {
		t.Fatalf("expected only selected note as source, got %+v", weekly.Sources)
	}
	if len(weekly.Files) != 1 || weekly.Files[0].Name != "local-review.md" {
		t.Fatalf("expected local file source, got %+v", weekly.Files)
	}
	if !strings.Contains(gen.lastPrompt, "local-review.md") || !strings.Contains(gen.lastPrompt, "本机文件复盘内容") {
		t.Fatalf("expected local file content in prompt, got %s", gen.lastPrompt)
	}
	if strings.Contains(gen.lastPrompt, displayTitle(unselected)) {
		t.Fatalf("expected unselected note to be excluded from prompt, got %s", gen.lastPrompt)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
