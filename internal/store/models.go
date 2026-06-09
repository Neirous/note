package store

import "time"

type Note struct {
	ID         int64     `json:"id"`
	ParentID   *int64    `json:"parent_id,omitempty"`
	Title      string    `json:"title"`
	Markdown   string    `json:"markdown"`
	HTML       string    `json:"html"`
	Tags       []string  `json:"tags,omitempty"`
	Status     string    `json:"status"`
	IsPinned   bool      `json:"is_pinned"`
	IsArchived bool      `json:"is_archived"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Chunk struct {
	ID        int64     `json:"id"`
	NoteID    int64     `json:"note_id"`
	Idx       int       `json:"idx"`
	Content   string    `json:"content"`
	Embedding []float64 `json:"embedding,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type NoteBlock struct {
	ID        int64     `json:"id"`
	NoteID    int64     `json:"note_id"`
	Position  int       `json:"position"`
	Level     int       `json:"level"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Checked   bool      `json:"checked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NoteBlockInput struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Checked bool   `json:"checked"`
	Level   int    `json:"level"`
}

type ReviewQuestion struct {
	ID        int64     `json:"id"`
	NoteID    int64     `json:"note_id"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReviewQuestionInput struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Source   string `json:"source"`
}

type NoteInsight struct {
	NoteID        int64     `json:"note_id"`
	Content       string    `json:"content"`
	NoteUpdatedAt time.Time `json:"note_updated_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type KnowledgeCard struct {
	ID             int64      `json:"id"`
	Front          string     `json:"front"`
	Back           string     `json:"back"`
	Tags           []string   `json:"tags,omitempty"`
	Status         string     `json:"status"`
	ReviewStage    int        `json:"review_stage"`
	LastReviewedAt *time.Time `json:"last_reviewed_at,omitempty"`
	NextReviewAt   *time.Time `json:"next_review_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type KnowledgeCardInput struct {
	Front  string   `json:"front"`
	Back   string   `json:"back"`
	Tags   []string `json:"tags"`
	Status string   `json:"status"`
}

type CardFilter struct {
	Query           string
	Status          string
	DueOnly         bool
	IncludeArchived bool
}

type ResearchSession struct {
	ID        int64     `json:"id"`
	Topic     string    `json:"topic"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}

type RecommendationSession struct {
	ID        int64     `json:"id"`
	Topic     string    `json:"topic"`
	Result    string    `json:"result"`
	CreatedAt time.Time `json:"created_at"`
}
