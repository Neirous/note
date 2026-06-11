package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *Store) ListKnowledgeCards(ctx context.Context, filter CardFilter) ([]KnowledgeCard, error) {
	var conds []string
	var args []any
	q := strings.TrimSpace(filter.Query)
	if q != "" {
		like := "%" + q + "%"
		conds = append(conds, "(front LIKE ? OR back LIKE ? OR tags LIKE ?)")
		args = append(args, like, like, like)
	}
	status := normalizeCardStatus(filter.Status)
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	} else if !filter.IncludeArchived {
		conds = append(conds, "status != 'archived'")
	}
	if filter.DueOnly {
		conds = append(conds, "status = 'active' AND (next_review_at IS NULL OR next_review_at <= CURRENT_TIMESTAMP)")
	}

	query := `
SELECT id, front, back, tags, status, review_stage, last_reviewed_at, next_review_at, created_at, updated_at
FROM knowledge_cards`
	if len(conds) > 0 {
		query += "\nWHERE " + strings.Join(conds, " AND ")
	}
	query += "\nORDER BY CASE WHEN next_review_at IS NULL THEN 0 ELSE 1 END ASC, next_review_at ASC, updated_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KnowledgeCard
	for rows.Next() {
		card, err := scanKnowledgeCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, card)
	}
	return out, rows.Err()
}

func (s *Store) GetKnowledgeCard(ctx context.Context, id int64) (KnowledgeCard, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, front, back, tags, status, review_stage, last_reviewed_at, next_review_at, created_at, updated_at
FROM knowledge_cards
WHERE id = ?`, id)
	card, err := scanKnowledgeCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return KnowledgeCard{}, ErrNotFound
	}
	if err != nil {
		return KnowledgeCard{}, err
	}
	return card, nil
}

func (s *Store) CreateKnowledgeCard(ctx context.Context, in KnowledgeCardInput) (KnowledgeCard, error) {
	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	if front == "" || back == "" {
		return KnowledgeCard{}, ErrInvalidState
	}
	tagsRaw, err := json.Marshal(normalizeTags(in.Tags))
	if err != nil {
		return KnowledgeCard{}, err
	}
	status := normalizeCardStatus(in.Status)
	if status == "" {
		status = "active"
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO knowledge_cards (front, back, tags, status, review_stage, next_review_at)
VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)`, front, back, string(tagsRaw), status)
	if err != nil {
		return KnowledgeCard{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return KnowledgeCard{}, err
	}
	return s.GetKnowledgeCard(ctx, id)
}

func (s *Store) UpdateKnowledgeCard(ctx context.Context, id int64, in KnowledgeCardInput) (KnowledgeCard, error) {
	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	if front == "" || back == "" {
		return KnowledgeCard{}, ErrInvalidState
	}
	tagsRaw, err := json.Marshal(normalizeTags(in.Tags))
	if err != nil {
		return KnowledgeCard{}, err
	}
	status := normalizeCardStatus(in.Status)
	if status == "" {
		status = "active"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE knowledge_cards
SET front = ?, back = ?, tags = ?, status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, front, back, string(tagsRaw), status, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeCard{}, err
	}
	if affected == 0 {
		return KnowledgeCard{}, ErrNotFound
	}
	return s.GetKnowledgeCard(ctx, id)
}

func (s *Store) DeleteKnowledgeCard(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_cards WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ReviewKnowledgeCard(ctx context.Context, id int64, remembered bool, now time.Time) (KnowledgeCard, error) {
	card, err := s.GetKnowledgeCard(ctx, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	stage := card.ReviewStage
	status := "active"
	if remembered {
		stage = card.ReviewStage + 1
		if stage >= len(ebbinghausIntervals) {
			stage = len(ebbinghausIntervals)
			status = "mastered"
		}
	}
	next := now
	if remembered && stage > 0 && stage <= len(ebbinghausIntervals) {
		next = now.Add(time.Duration(ebbinghausIntervals[stage-1]) * 24 * time.Hour)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE knowledge_cards
SET status = ?, review_stage = ?, last_reviewed_at = ?, next_review_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, status, stage, now, next, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeCard{}, err
	}
	if affected == 0 {
		return KnowledgeCard{}, ErrNotFound
	}
	return s.GetKnowledgeCard(ctx, id)
}
