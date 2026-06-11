package grpcserver

import (
	"context"

	ragpb "note/api/proto/rag/v1"
	"note/internal/rag"
)

type RAGServer struct {
	ragpb.UnimplementedRAGServiceServer
	svc *rag.Service
}

func NewRAGServer(svc *rag.Service) *RAGServer {
	return &RAGServer{svc: svc}
}

func (s *RAGServer) Search(ctx context.Context, req *ragpb.SearchRequest) (*ragpb.SearchResponse, error) {
	topK := int(req.TopK)
	result, err := s.svc.SearchWithOptions(ctx, req.Query, topK, rag.SearchOptions{AnchorNoteID: req.AnchorNoteId})
	if err != nil {
		return nil, err
	}
	pb := make([]*ragpb.ChunkWithScore, len(result.Results))
	for i, r := range result.Results {
		pb[i] = &ragpb.ChunkWithScore{
			NoteId: r.NoteID, NoteTitle: r.NoteTitle, ChunkId: r.ChunkID,
			Index: int32(r.Index), Content: r.Content, Score: r.Score,
		}
	}
	return &ragpb.SearchResponse{Query: req.Query, Results: pb}, nil
}

func (s *RAGServer) Ask(ctx context.Context, req *ragpb.AskRequest) (*ragpb.AskResponse, error) {
	result, err := s.svc.AskWithOptions(ctx, req.Query, int(req.TopK), rag.SearchOptions{AnchorNoteID: req.AnchorNoteId})
	if err != nil {
		return nil, err
	}
	pb := make([]*ragpb.ChunkWithScore, len(result.Contexts))
	for i, c := range result.Contexts {
		pb[i] = &ragpb.ChunkWithScore{
			NoteId: c.NoteID, NoteTitle: c.NoteTitle, ChunkId: c.ChunkID,
			Index: int32(c.Index), Content: c.Content, Score: c.Score,
		}
	}
	return &ragpb.AskResponse{Query: req.Query, Answer: result.Answer, Contexts: pb}, nil
}

func (s *RAGServer) IndexNote(ctx context.Context, req *ragpb.IndexNoteRequest) (*ragpb.IndexNoteResponse, error) {
	if err := s.svc.IndexNote(ctx, req.NoteId, req.Markdown); err != nil {
		return nil, err
	}
	return &ragpb.IndexNoteResponse{Ok: true}, nil
}
