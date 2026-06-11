package grpcserver

import (
	"context"
	"strings"

	commonv1 "note/api/proto/common/v1"
	intelligencepb "note/api/proto/intelligence/v1"
	"note/internal/rag"
	"note/internal/store"
)

type IntelligenceServer struct {
	intelligencepb.UnimplementedIntelligenceServiceServer
	store *store.Store
	rag   *rag.Service
}

func NewIntelligenceServer(st *store.Store, ragSvc *rag.Service) *IntelligenceServer {
	return &IntelligenceServer{store: st, rag: ragSvc}
}

func (s *IntelligenceServer) GetNoteInsights(ctx context.Context, req *intelligencepb.GetNoteInsightsRequest) (*intelligencepb.NoteInsightsResponse, error) {
	resp := &intelligencepb.NoteInsightsResponse{}
	if s.rag != nil {
		note, err := s.store.GetNote(ctx, req.NoteId)
		if err != nil {
			return nil, err
		}
		prompt := "分析以下笔记，用JSON返回 summary, outline, keywords, suggested_tags, quality_score(1-100), quality_issues。笔记：" + note.Markdown
		raw, err := s.rag.Generate(ctx, prompt)
		if err == nil && raw != "" {
			resp.UsedAi = true
			resp.Summary = raw
		}
	}
	return resp, nil
}

func (s *IntelligenceServer) SuggestTags(ctx context.Context, req *intelligencepb.SuggestTagsRequest) (*intelligencepb.SuggestTagsResponse, error) {
	note, err := s.store.GetNote(ctx, req.NoteId)
	if err != nil {
		return nil, err
	}
	tags := extractKeywords(note.Title + " " + note.Markdown)
	return &intelligencepb.SuggestTagsResponse{Tags: tags, UsedAi: false}, nil
}

func (s *IntelligenceServer) GetNoteRecommendations(ctx context.Context, req *intelligencepb.GetNoteRecommendationsRequest) (*intelligencepb.NoteRecommendationsResponse, error) {
	notes, err := s.store.ListNotes(ctx, store.NoteFilter{IncludeArchived: false})
	if err != nil {
		return nil, err
	}
	result := make([]*commonv1.NoteReference, 0, min(5, len(notes)))
	for i := range notes {
		if notes[i].ID != req.NoteId && len(result) < 5 {
			result = append(result, noteRefToProto(&notes[i]))
		}
	}
	return &intelligencepb.NoteRecommendationsResponse{Notes: result}, nil
}

func (s *IntelligenceServer) GetNoteLinks(ctx context.Context, req *intelligencepb.GetNoteLinksRequest) (*intelligencepb.NoteLinksResponse, error) {
	return &intelligencepb.NoteLinksResponse{}, nil
}

func (s *IntelligenceServer) ListReviewQuestions(ctx context.Context, req *intelligencepb.ListReviewQuestionsRequest) (*intelligencepb.ListReviewQuestionsResponse, error) {
	qs, err := s.store.ListReviewQuestions(ctx, req.NoteId)
	if err != nil {
		return nil, err
	}
	pb := make([]*commonv1.ReviewQuestion, len(qs))
	for i, q := range qs {
		pb[i] = &commonv1.ReviewQuestion{Id: q.ID, NoteId: q.NoteID, Question: q.Question, Answer: q.Answer, Source: q.Source}
	}
	return &intelligencepb.ListReviewQuestionsResponse{Questions: pb}, nil
}

func (s *IntelligenceServer) CreateReviewQuestion(ctx context.Context, req *intelligencepb.CreateReviewQuestionRequest) (*commonv1.ReviewQuestion, error) {
	q, err := s.store.CreateReviewQuestion(ctx, req.NoteId, store.ReviewQuestionInput{
		Question: req.Question, Answer: req.Answer, Source: req.Source,
	})
	if err != nil {
		return nil, err
	}
	return &commonv1.ReviewQuestion{Id: q.ID, NoteId: q.NoteID, Question: q.Question, Answer: q.Answer, Source: q.Source}, nil
}

func (s *IntelligenceServer) GenerateReviewQuestions(ctx context.Context, req *intelligencepb.GenerateReviewQuestionsRequest) (*intelligencepb.GenerateReviewQuestionsResponse, error) {
	if s.rag == nil {
		return &intelligencepb.GenerateReviewQuestionsResponse{}, nil
	}
	note, err := s.store.GetNote(ctx, req.NoteId)
	if err != nil {
		return nil, err
	}
	prompt := "根据以下笔记生成复习问题，返回JSON数组[{\"question\":\"...\",\"answer\":\"...\"}]。笔记：" + note.Markdown
	raw, _ := s.rag.Generate(ctx, prompt)
	_ = raw
	return &intelligencepb.GenerateReviewQuestionsResponse{}, nil
}

func (s *IntelligenceServer) UpdateReviewQuestion(ctx context.Context, req *intelligencepb.UpdateReviewQuestionRequest) (*commonv1.ReviewQuestion, error) {
	q, err := s.store.UpdateReviewQuestion(ctx, req.NoteId, req.QuestionId, store.ReviewQuestionInput{
		Question: req.Question, Answer: req.Answer,
	})
	if err != nil {
		return nil, err
	}
	return &commonv1.ReviewQuestion{Id: q.ID, NoteId: q.NoteID, Question: q.Question, Answer: q.Answer, Source: q.Source}, nil
}

func (s *IntelligenceServer) DeleteReviewQuestion(ctx context.Context, req *intelligencepb.DeleteReviewQuestionRequest) (*intelligencepb.DeleteReviewQuestionResponse, error) {
	if err := s.store.DeleteReviewQuestion(ctx, req.NoteId, req.QuestionId); err != nil {
		return nil, err
	}
	return &intelligencepb.DeleteReviewQuestionResponse{Ok: true}, nil
}

func (s *IntelligenceServer) GetDailyReview(ctx context.Context, req *intelligencepb.GetDailyReviewRequest) (*intelligencepb.DailyReviewResponse, error) {
	notes, _ := s.store.ListNotes(ctx, store.NoteFilter{IncludeArchived: false})
	refs := make([]*commonv1.NoteReference, 0, min(5, len(notes)))
	for i := range notes {
		if len(refs) >= 5 {
			break
		}
		refs = append(refs, noteRefToProto(&notes[i]))
	}
	return &intelligencepb.DailyReviewResponse{RecommendedNext: refs}, nil
}

func (s *IntelligenceServer) CreateAIRecommendation(ctx context.Context, req *intelligencepb.CreateAIRecommendationRequest) (*intelligencepb.AIRecommendationResponse, error) {
	return &intelligencepb.AIRecommendationResponse{Topic: req.Topic, Summary: "AI recommendation not available"}, nil
}

func (s *IntelligenceServer) ListRecommendationSessions(ctx context.Context, req *intelligencepb.ListRecommendationSessionsRequest) (*intelligencepb.ListRecommendationSessionsResponse, error) {
	sessions, _ := s.store.ListRecommendationSessions(ctx, 50)
	pb := make([]*intelligencepb.RecommendationSessionHistoryItem, len(sessions))
	for i, sess := range sessions {
		pb[i] = &intelligencepb.RecommendationSessionHistoryItem{
			Id: sess.ID, Topic: sess.Topic, CreatedAt: sess.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return &intelligencepb.ListRecommendationSessionsResponse{Sessions: pb}, nil
}

func (s *IntelligenceServer) DeleteRecommendationSession(ctx context.Context, req *intelligencepb.DeleteRecommendationSessionRequest) (*intelligencepb.DeleteRecommendationSessionResponse, error) {
	s.store.DeleteRecommendationSession(ctx, req.Id)
	return &intelligencepb.DeleteRecommendationSessionResponse{Ok: true}, nil
}

func (s *IntelligenceServer) CreateResearchSession(ctx context.Context, req *intelligencepb.CreateResearchSessionRequest) (*intelligencepb.ResearchSessionResponse, error) {
	return &intelligencepb.ResearchSessionResponse{Topic: req.Topic}, nil
}

func (s *IntelligenceServer) ListResearchSessions(ctx context.Context, req *intelligencepb.ListResearchSessionsRequest) (*intelligencepb.ListResearchSessionsResponse, error) {
	sessions, _ := s.store.ListResearchSessions(ctx, 50)
	pb := make([]*intelligencepb.ResearchSessionHistoryItem, len(sessions))
	for i, sess := range sessions {
		pb[i] = &intelligencepb.ResearchSessionHistoryItem{
			Id: sess.ID, Topic: sess.Topic, CreatedAt: sess.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return &intelligencepb.ListResearchSessionsResponse{Sessions: pb}, nil
}

func (s *IntelligenceServer) DeleteResearchSession(ctx context.Context, req *intelligencepb.DeleteResearchSessionRequest) (*intelligencepb.DeleteResearchSessionResponse, error) {
	s.store.DeleteResearchSession(ctx, req.Id)
	return &intelligencepb.DeleteResearchSessionResponse{Ok: true}, nil
}

func (s *IntelligenceServer) CreateWeeklyReport(ctx context.Context, req *intelligencepb.CreateWeeklyReportRequest) (*intelligencepb.WeeklyReportResponse, error) {
	return &intelligencepb.WeeklyReportResponse{}, nil
}

func (s *IntelligenceServer) OptimizeNote(ctx context.Context, req *intelligencepb.OptimizeNoteRequest) (*intelligencepb.OptimizeNoteResponse, error) {
	if s.rag == nil {
		return &intelligencepb.OptimizeNoteResponse{Markdown: req.Markdown}, nil
	}
	prompt := "请优化以下笔记，只输出优化后的Markdown：\n" + req.Markdown
	optimized, _ := s.rag.Generate(ctx, prompt)
	if strings.TrimSpace(optimized) == "" {
		optimized = req.Markdown
	}
	return &intelligencepb.OptimizeNoteResponse{Markdown: optimized}, nil
}

func extractKeywords(text string) []string {
	words := strings.Fields(text)
	seen := map[string]struct{}{}
	var out []string
	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,;:!?()[]{}\"'"))
		if len(w) >= 2 && len(w) <= 20 {
			if _, ok := seen[w]; !ok {
				seen[w] = struct{}{}
				out = append(out, w)
				if len(out) >= 10 {
					break
				}
			}
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
