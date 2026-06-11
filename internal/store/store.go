package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrInvalidState = errors.New("invalid note state")
var ErrInvalidBlock = errors.New("invalid note block")

type NoteFilter struct {
	Query           string
	Tag             string
	IncludeArchived bool
	OnlyArchived    bool
}

type NoteInput struct {
	ParentID *int64
	Title    string
	Markdown string
	HTML     string
	Tags     []string
	Status   string
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InitSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER NULL REFERENCES notes(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    markdown TEXT NOT NULL,
    html TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unfinished',
    is_pinned INTEGER NOT NULL DEFAULT 0,
    is_archived INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS note_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id INTEGER NOT NULL,
    idx INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_tags (
    note_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    PRIMARY KEY (note_id, tag),
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_blocks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    level INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    checked INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_review_questions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id INTEGER NOT NULL,
    question TEXT NOT NULL,
    answer TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'manual',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_insights (
    note_id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    note_updated_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS knowledge_cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',
    review_stage INTEGER NOT NULL DEFAULT 0,
    last_reviewed_at DATETIME NULL,
    next_review_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS research_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    result TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recommendation_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    result TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chunks_note_id ON note_chunks(note_id);
CREATE INDEX IF NOT EXISTS idx_note_tags_tag ON note_tags(tag);
CREATE INDEX IF NOT EXISTS idx_note_blocks_note_pos ON note_blocks(note_id, position);
CREATE INDEX IF NOT EXISTS idx_note_review_questions_note_id ON note_review_questions(note_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_note_insights_updated ON note_insights(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_cards_status_due ON knowledge_cards(status, next_review_at);
CREATE INDEX IF NOT EXISTS idx_research_sessions_created ON research_sessions(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_recommendation_sessions_created ON recommendation_sessions(created_at DESC, id DESC);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}

	hasParentID, err := s.hasColumn(ctx, "notes", "parent_id")
	if err != nil {
		return err
	}
	if !hasParentID {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN parent_id INTEGER NULL REFERENCES notes(id) ON DELETE SET NULL`); err != nil {
			return fmt.Errorf("add parent_id column: %w", err)
		}
	}
	hasPinned, err := s.hasColumn(ctx, "notes", "is_pinned")
	if err != nil {
		return err
	}
	if !hasPinned {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN is_pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add is_pinned column: %w", err)
		}
	}
	hasArchived, err := s.hasColumn(ctx, "notes", "is_archived")
	if err != nil {
		return err
	}
	if !hasArchived {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN is_archived INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add is_archived column: %w", err)
		}
	}
	hasStatus, err := s.hasColumn(ctx, "notes", "status")
	if err != nil {
		return err
	}
	if !hasStatus {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN status TEXT NOT NULL DEFAULT 'unfinished'`); err != nil {
			return fmt.Errorf("add status column: %w", err)
		}
	}
	hasBlockLevel, err := s.hasColumn(ctx, "note_blocks", "level")
	if err != nil {
		return err
	}
	if !hasBlockLevel {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE note_blocks ADD COLUMN level INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add note_blocks.level column: %w", err)
		}
	}

	return nil
}

func (s *Store) hasColumn(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) NowUTC() time.Time {
	return time.Now().UTC()
}

// ---- helpers ----

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := normalizeTag(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func normalizeNoteStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "completed":
		return "completed"
	default:
		return "unfinished"
	}
}

func normalizeCardStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "active", "archived", "mastered":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func normalizeBlockType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "paragraph", "todo", "heading1", "code", "quote", "table":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func normalizeBlockLevel(v int) int {
	if v < 0 {
		return 0
	}
	if v > 6 {
		return 6
	}
	return v
}

func normalizeReviewQuestionSource(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ai":
		return "ai"
	default:
		return "manual"
	}
}

func replaceTagsTx(ctx context.Context, tx *sql.Tx, noteID int64, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM note_tags WHERE note_id = ?`, noteID); err != nil {
		return err
	}

	tags = normalizeTags(tags)
	if len(tags) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO note_tags (note_id, tag) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.ExecContext(ctx, noteID, tag); err != nil {
			return err
		}
	}
	return nil
}

type knowledgeCardScanner interface {
	Scan(dest ...any) error
}

var ebbinghausIntervals = []int{1, 2, 4, 7, 15, 30}

func scanKnowledgeCard(row knowledgeCardScanner) (KnowledgeCard, error) {
	var (
		card           KnowledgeCard
		rawTags        string
		lastReviewedAt sql.NullTime
		nextReviewAt   sql.NullTime
	)
	if err := row.Scan(&card.ID, &card.Front, &card.Back, &rawTags, &card.Status, &card.ReviewStage, &lastReviewedAt, &nextReviewAt, &card.CreatedAt, &card.UpdatedAt); err != nil {
		return KnowledgeCard{}, err
	}
	card.Status = normalizeCardStatus(card.Status)
	if card.Status == "" {
		card.Status = "active"
	}
	if lastReviewedAt.Valid {
		card.LastReviewedAt = &lastReviewedAt.Time
	}
	if nextReviewAt.Valid {
		card.NextReviewAt = &nextReviewAt.Time
	}
	_ = json.Unmarshal([]byte(rawTags), &card.Tags)
	card.Tags = normalizeTags(card.Tags)
	return card, nil
}

func (s *Store) tagsByNoteIDs(ctx context.Context, idsInput []int64) (map[int64][]string, error) {
	out := make(map[int64][]string)
	if len(idsInput) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(idsInput))
	args := make([]any, 0, len(idsInput))
	for _, id := range idsInput {
		ids = append(ids, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(`
SELECT note_id, tag
FROM note_tags
WHERE note_id IN (%s)
ORDER BY tag ASC`, strings.Join(ids, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			noteID int64
			tag    string
		)
		if err := rows.Scan(&noteID, &tag); err != nil {
			return nil, err
		}
		out[noteID] = append(out[noteID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
