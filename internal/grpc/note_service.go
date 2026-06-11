package grpcserver

import (
	"context"
	"fmt"
	"strings"

	commonv1 "note/api/proto/common/v1"
	notepb "note/api/proto/note/v1"
	"note/internal/rag"
	"note/internal/store"
)

type NoteServer struct {
	notepb.UnimplementedNoteServiceServer
	store *store.Store
	rag   *rag.Service
}

func NewNoteServer(st *store.Store, ragSvc *rag.Service) *NoteServer {
	return &NoteServer{store: st, rag: ragSvc}
}

func (s *NoteServer) ListNotes(ctx context.Context, req *notepb.ListNotesRequest) (*notepb.ListNotesResponse, error) {
	notes, err := s.store.ListNotes(ctx, store.NoteFilter{
		Query: req.Query, Tag: req.Tag,
		IncludeArchived: req.IncludeArchived, OnlyArchived: req.OnlyArchived,
	})
	if err != nil {
		return nil, err
	}
	pb := make([]*commonv1.Note, len(notes))
	for i, n := range notes {
		pb[i] = noteToProto(&n)
	}
	return &notepb.ListNotesResponse{Notes: pb}, nil
}

func (s *NoteServer) GetNote(ctx context.Context, req *notepb.GetNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.GetNote(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) CreateNote(ctx context.Context, req *notepb.CreateNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: int64Ptr(req.ParentId),
		Title:    req.Title, Markdown: req.Markdown, Tags: req.Tags, HTML: "",
	})
	if err != nil {
		return nil, err
	}
	_ = s.rag.IndexNote(ctx, n.ID, n.Markdown)
	return noteToProto(&n), nil
}

func (s *NoteServer) UpdateNote(ctx context.Context, req *notepb.UpdateNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.UpdateNote(ctx, req.Id, store.NoteInput{
		ParentID: int64Ptr(req.ParentId),
		Title:    req.Title, Markdown: req.Markdown, Tags: req.Tags,
	})
	if err != nil {
		return nil, err
	}
	_ = s.rag.IndexNote(ctx, n.ID, n.Markdown)
	return noteToProto(&n), nil
}

func (s *NoteServer) DeleteNote(ctx context.Context, req *notepb.DeleteNoteRequest) (*notepb.DeleteNoteResponse, error) {
	if err := s.store.DeleteNote(ctx, req.Id); err != nil {
		return nil, err
	}
	return &notepb.DeleteNoteResponse{Ok: true}, nil
}

func (s *NoteServer) PinNote(ctx context.Context, req *notepb.PinNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.SetPinned(ctx, req.Id, req.Value)
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) ArchiveNote(ctx context.Context, req *notepb.ArchiveNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.SetArchived(ctx, req.Id, req.Value)
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) SetNoteStatus(ctx context.Context, req *notepb.SetNoteStatusRequest) (*commonv1.Note, error) {
	n, err := s.store.SetNoteStatus(ctx, req.Id, req.Status)
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) DuplicateNote(ctx context.Context, req *notepb.DuplicateNoteRequest) (*commonv1.Note, error) {
	src, err := s.store.GetNote(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: src.ParentID, Tags: src.Tags,
		Title: fmt.Sprintf("Copy of %s", src.Title), Markdown: src.Markdown, HTML: src.HTML,
	})
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) ExportNote(ctx context.Context, req *notepb.ExportNoteRequest) (*notepb.ExportNoteResponse, error) {
	n, err := s.store.GetNote(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &notepb.ExportNoteResponse{Markdown: n.Markdown, Filename: sanitizeFilename(n.Title)}, nil
}

func (s *NoteServer) ListTags(ctx context.Context, req *notepb.ListTagsRequest) (*notepb.ListTagsResponse, error) {
	tags, err := s.store.ListDistinctTags(ctx)
	if err != nil {
		return nil, err
	}
	return &notepb.ListTagsResponse{Tags: tags}, nil
}

func (s *NoteServer) DeleteTag(ctx context.Context, req *notepb.DeleteTagRequest) (*notepb.DeleteTagResponse, error) {
	if err := s.store.DeleteTag(ctx, req.Tag); err != nil {
		return nil, err
	}
	return &notepb.DeleteTagResponse{Ok: true}, nil
}

func (s *NoteServer) ListBlocks(ctx context.Context, req *notepb.ListBlocksRequest) (*notepb.ListBlocksResponse, error) {
	blocks, err := s.store.ListNoteBlocks(ctx, req.NoteId)
	if err != nil {
		return nil, err
	}
	pb := make([]*commonv1.NoteBlock, len(blocks))
	for i, b := range blocks {
		pb[i] = blockToProto(&b)
	}
	return &notepb.ListBlocksResponse{Blocks: pb}, nil
}

func (s *NoteServer) ReplaceBlocks(ctx context.Context, req *notepb.ReplaceBlocksRequest) (*notepb.ReplaceBlocksResponse, error) {
	inputs := make([]store.NoteBlockInput, len(req.Blocks))
	for i, b := range req.Blocks {
		inputs[i] = store.NoteBlockInput{Type: b.Type, Content: b.Content, Checked: b.Checked, Level: int(b.Level)}
	}
	if err := s.store.ReplaceNoteBlocks(ctx, req.NoteId, inputs); err != nil {
		return nil, err
	}
	blocks, _ := s.store.ListNoteBlocks(ctx, req.NoteId)
	pb := make([]*commonv1.NoteBlock, len(blocks))
	for i, b := range blocks {
		pb[i] = blockToProto(&b)
	}
	n, _ := s.store.GetNote(ctx, req.NoteId)
	return &notepb.ReplaceBlocksResponse{Note: noteToProto(&n), Blocks: pb}, nil
}

func (s *NoteServer) ListTemplates(ctx context.Context, req *notepb.ListTemplatesRequest) (*notepb.ListTemplatesResponse, error) {
	templates := []*notepb.Template{
		{Key: "study", Title: "学习笔记", Description: "包含学习目标、重点、总结的学习笔记模板"},
		{Key: "meeting", Title: "会议记录", Description: "会议议程、讨论要点、行动项"},
		{Key: "project", Title: "项目计划", Description: "目标、里程碑、任务清单"},
		{Key: "weekly", Title: "周报", Description: "本周总结与下周计划"},
	}
	return &notepb.ListTemplatesResponse{Templates: templates}, nil
}

func (s *NoteServer) CreateFromTemplate(ctx context.Context, req *notepb.CreateFromTemplateRequest) (*commonv1.Note, error) {
	key := strings.ToLower(strings.TrimSpace(req.Key))
	templates := map[string]string{
		"study":   "# 学习笔记\n\n## 学习目标\n\n## 重点内容\n\n## 总结\n",
		"meeting": "# 会议记录\n\n## 议程\n\n## 讨论要点\n\n## 行动项\n- [ ] \n",
		"project": "# 项目计划\n\n## 目标\n\n## 里程碑\n\n## 任务清单\n- [ ] \n",
		"weekly":  "# 周报\n\n## 本周完成\n\n## 遇到的问题\n\n## 下周计划\n",
	}
	md := templates[key]
	if md == "" {
		md = "# New Note\n"
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Untitled"
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{Title: title, Markdown: md, HTML: ""})
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) ImportNote(ctx context.Context, req *notepb.ImportNoteRequest) (*commonv1.Note, error) {
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		Title: req.Title, Markdown: req.Markdown, Tags: req.Tags, HTML: "",
	})
	if err != nil {
		return nil, err
	}
	return noteToProto(&n), nil
}

func (s *NoteServer) RenderMarkdown(ctx context.Context, req *notepb.RenderMarkdownRequest) (*notepb.RenderMarkdownResponse, error) {
	return &notepb.RenderMarkdownResponse{Html: req.Markdown}, nil
}

func (s *NoteServer) ListTasks(ctx context.Context, req *notepb.ListTasksRequest) (*notepb.ListTasksResponse, error) {
	return &notepb.ListTasksResponse{}, nil
}

// ---- Converters ----

func noteToProto(n *store.Note) *commonv1.Note {
	pb := &commonv1.Note{
		Id: n.ID, Title: n.Title, Markdown: n.Markdown, Html: n.HTML,
		Tags: n.Tags, Status: n.Status, IsPinned: n.IsPinned, IsArchived: n.IsArchived,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if n.ParentID != nil {
		pb.ParentId = *n.ParentID
	}
	return pb
}

func blockToProto(b *store.NoteBlock) *commonv1.NoteBlock {
	return &commonv1.NoteBlock{
		Id: b.ID, NoteId: b.NoteID, Position: int32(b.Position),
		Level: int32(b.Level), Type: b.Type, Content: b.Content, Checked: b.Checked,
	}
}

func noteRefToProto(n *store.Note) *commonv1.NoteReference {
	return &commonv1.NoteReference{Id: n.ID, Title: n.Title, Tags: n.Tags}
}

func int64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func sanitizeFilename(s string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, s)
	return strings.Trim(result, "-")
}
