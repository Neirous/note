package grpcserver

import (
	"context"
	"time"

	commonv1 "note/api/proto/common/v1"
	knowledgepb "note/api/proto/knowledge/v1"
	"note/internal/store"
)

type KnowledgeServer struct {
	knowledgepb.UnimplementedKnowledgeServiceServer
	store *store.Store
}

func NewKnowledgeServer(st *store.Store) *KnowledgeServer {
	return &KnowledgeServer{store: st}
}

func (s *KnowledgeServer) ListCards(ctx context.Context, req *knowledgepb.ListCardsRequest) (*knowledgepb.ListCardsResponse, error) {
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{
		Query: req.Query, Status: req.Status, IncludeArchived: req.IncludeArchived,
	})
	if err != nil {
		return nil, err
	}
	pb := make([]*commonv1.KnowledgeCard, len(cards))
	for i, c := range cards {
		pb[i] = cardToProto(&c)
	}
	return &knowledgepb.ListCardsResponse{Cards: pb}, nil
}

func (s *KnowledgeServer) ListDueCards(ctx context.Context, req *knowledgepb.ListDueCardsRequest) (*knowledgepb.ListDueCardsResponse, error) {
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{DueOnly: true})
	if err != nil {
		return nil, err
	}
	pb := make([]*commonv1.KnowledgeCard, len(cards))
	for i, c := range cards {
		pb[i] = cardToProto(&c)
	}
	return &knowledgepb.ListDueCardsResponse{Cards: pb}, nil
}

func (s *KnowledgeServer) GetCard(ctx context.Context, req *knowledgepb.GetCardRequest) (*commonv1.KnowledgeCard, error) {
	c, err := s.store.GetKnowledgeCard(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return cardToProto(&c), nil
}

func (s *KnowledgeServer) CreateCard(ctx context.Context, req *knowledgepb.CreateCardRequest) (*commonv1.KnowledgeCard, error) {
	c, err := s.store.CreateKnowledgeCard(ctx, store.KnowledgeCardInput{
		Front: req.Front, Back: req.Back, Tags: req.Tags, Status: req.Status,
	})
	if err != nil {
		return nil, err
	}
	return cardToProto(&c), nil
}

func (s *KnowledgeServer) UpdateCard(ctx context.Context, req *knowledgepb.UpdateCardRequest) (*commonv1.KnowledgeCard, error) {
	c, err := s.store.UpdateKnowledgeCard(ctx, req.Id, store.KnowledgeCardInput{
		Front: req.Front, Back: req.Back, Tags: req.Tags, Status: req.Status,
	})
	if err != nil {
		return nil, err
	}
	return cardToProto(&c), nil
}

func (s *KnowledgeServer) DeleteCard(ctx context.Context, req *knowledgepb.DeleteCardRequest) (*knowledgepb.DeleteCardResponse, error) {
	if err := s.store.DeleteKnowledgeCard(ctx, req.Id); err != nil {
		return nil, err
	}
	return &knowledgepb.DeleteCardResponse{Ok: true}, nil
}

func (s *KnowledgeServer) ReviewCard(ctx context.Context, req *knowledgepb.ReviewCardRequest) (*knowledgepb.ReviewCardResponse, error) {
	c, err := s.store.ReviewKnowledgeCard(ctx, req.Id, req.Remembered, time.Now())
	if err != nil {
		return nil, err
	}
	return &knowledgepb.ReviewCardResponse{Card: cardToProto(&c), Stage: int32(c.ReviewStage), Mastered: c.Status == "mastered"}, nil
}

func cardToProto(c *store.KnowledgeCard) *commonv1.KnowledgeCard {
	pb := &commonv1.KnowledgeCard{
		Id: c.ID, Front: c.Front, Back: c.Back, Tags: c.Tags,
		Status: c.Status, ReviewStage: int32(c.ReviewStage),
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if c.LastReviewedAt != nil {
		pb.LastReviewedAt = c.LastReviewedAt.Format("2006-01-02T15:04:05Z")
	}
	if c.NextReviewAt != nil {
		pb.NextReviewAt = c.NextReviewAt.Format("2006-01-02T15:04:05Z")
	}
	return pb
}
