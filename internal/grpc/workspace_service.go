package grpcserver

import (
	"context"

	commonv1 "note/api/proto/common/v1"
	workspacepb "note/api/proto/workspace/v1"
	"note/internal/rag"
	"note/internal/store"
)

type WorkspaceServer struct {
	workspacepb.UnimplementedWorkspaceServiceServer
	store *store.Store
	rag   *rag.Service
}

func NewWorkspaceServer(st *store.Store, ragSvc *rag.Service) *WorkspaceServer {
	return &WorkspaceServer{store: st, rag: ragSvc}
}

func (s *WorkspaceServer) GetDashboard(ctx context.Context, req *workspacepb.GetDashboardRequest) (*workspacepb.DashboardResponse, error) {
	notes, _ := s.store.ListNotes(ctx, store.NoteFilter{IncludeArchived: false})
	cards, _ := s.store.ListKnowledgeCards(ctx, store.CardFilter{})

	unfinished := 0
	completed := 0
	for _, n := range notes {
		switch n.Status {
		case "completed":
			completed++
		default:
			unfinished++
		}
	}

	unfinishedRefs := make([]*commonv1.NoteReference, 0)
	recentRefs := make([]*commonv1.NoteReference, 0)
	for i := range notes {
		ref := noteRefToProto(&notes[i])
		if notes[i].Status == "unfinished" && len(unfinishedRefs) < 5 {
			unfinishedRefs = append(unfinishedRefs, ref)
		}
		if len(recentRefs) < 5 {
			recentRefs = append(recentRefs, ref)
		}
	}

	return &workspacepb.DashboardResponse{
		Stats: map[string]int32{
			"total_notes":      int32(len(notes)),
			"total_cards":      int32(len(cards)),
			"unfinished_notes": int32(unfinished),
			"completed_notes":  int32(completed),
		},
		NoteStatusPie: []*workspacepb.ChartPoint{
			{Label: "已完成", Value: int32(completed)},
			{Label: "未完成", Value: int32(unfinished)},
		},
		UnfinishedNotes: unfinishedRefs,
		RecentActivity:  recentRefs,
	}, nil
}

func (s *WorkspaceServer) GetGraph(ctx context.Context, req *workspacepb.GetGraphRequest) (*workspacepb.GraphResponse, error) {
	notes, _ := s.store.ListNotesLite(ctx, store.NoteFilter{})
	nodes := make([]*workspacepb.GraphNode, len(notes))
	for i, n := range notes {
		nodes[i] = &workspacepb.GraphNode{
			Id: n.ID, Title: n.Title, Tags: n.Tags,
			UpdatedAt: n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return &workspacepb.GraphResponse{Nodes: nodes}, nil
}

func (s *WorkspaceServer) EvaluateQuality(ctx context.Context, req *workspacepb.EvaluateQualityRequest) (*workspacepb.QualityEvaluationResponse, error) {
	return &workspacepb.QualityEvaluationResponse{UsedAi: false}, nil
}
