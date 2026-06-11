package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ---- Review Questions ----

func (s *Store) ListReviewQuestions(ctx context.Context, noteID int64) ([]ReviewQuestion, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, note_id, question, answer, source, created_at, updated_at
FROM note_review_questions
WHERE note_id = ?
ORDER BY updated_at DESC, id DESC`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReviewQuestion
	for rows.Next() {
		var q ReviewQuestion
		if err := rows.Scan(&q.ID, &q.NoteID, &q.Question, &q.Answer, &q.Source, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) CreateReviewQuestion(ctx context.Context, noteID int64, in ReviewQuestionInput) (ReviewQuestion, error) {
	if _, err := s.GetNote(ctx, noteID); err != nil {
		return ReviewQuestion{}, err
	}
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return ReviewQuestion{}, ErrInvalidState
	}
	answer := strings.TrimSpace(in.Answer)
	source := normalizeReviewQuestionSource(in.Source)

	res, err := s.db.ExecContext(ctx, `
INSERT INTO note_review_questions (note_id, question, answer, source)
VALUES (?, ?, ?, ?)`, noteID, question, answer, source)
	if err != nil {
		return ReviewQuestion{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ReviewQuestion{}, err
	}
	return s.GetReviewQuestion(ctx, noteID, id)
}

func (s *Store) GetReviewQuestion(ctx context.Context, noteID, id int64) (ReviewQuestion, error) {
	var q ReviewQuestion
	err := s.db.QueryRowContext(ctx, `
SELECT id, note_id, question, answer, source, created_at, updated_at
FROM note_review_questions
WHERE note_id = ? AND id = ?`, noteID, id).Scan(&q.ID, &q.NoteID, &q.Question, &q.Answer, &q.Source, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ReviewQuestion{}, ErrNotFound
	}
	if err != nil {
		return ReviewQuestion{}, err
	}
	return q, nil
}

func (s *Store) UpdateReviewQuestion(ctx context.Context, noteID, id int64, in ReviewQuestionInput) (ReviewQuestion, error) {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return ReviewQuestion{}, ErrInvalidState
	}
	answer := strings.TrimSpace(in.Answer)

	res, err := s.db.ExecContext(ctx, `
UPDATE note_review_questions
SET question = ?, answer = ?, updated_at = CURRENT_TIMESTAMP
WHERE note_id = ? AND id = ?`, question, answer, noteID, id)
	if err != nil {
		return ReviewQuestion{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ReviewQuestion{}, err
	}
	if affected == 0 {
		return ReviewQuestion{}, ErrNotFound
	}
	return s.GetReviewQuestion(ctx, noteID, id)
}

func (s *Store) DeleteReviewQuestion(ctx context.Context, noteID, id int64) error {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM note_review_questions
WHERE note_id = ? AND id = ?`, noteID, id)
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

// ---- Note Insights ----

func (s *Store) GetNoteInsight(ctx context.Context, noteID int64) (NoteInsight, error) {
	var insight NoteInsight
	row := s.db.QueryRowContext(ctx, `
SELECT note_id, content, note_updated_at, created_at, updated_at
FROM note_insights
WHERE note_id = ?`, noteID)
	if err := row.Scan(&insight.NoteID, &insight.Content, &insight.NoteUpdatedAt, &insight.CreatedAt, &insight.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return NoteInsight{}, ErrNotFound
	} else if err != nil {
		return NoteInsight{}, err
	}
	return insight, nil
}

func (s *Store) SaveNoteInsight(ctx context.Context, noteID int64, noteUpdatedAt time.Time, content []byte) error {
	if noteID <= 0 || len(content) == 0 {
		return ErrInvalidState
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO note_insights (note_id, content, note_updated_at)
VALUES (?, ?, ?)
ON CONFLICT(note_id) DO UPDATE SET
    content = excluded.content,
    note_updated_at = excluded.note_updated_at,
    updated_at = CURRENT_TIMESTAMP`, noteID, string(content), noteUpdatedAt)
	return err
}

// ---- Research Sessions ----

func (s *Store) CreateResearchSession(ctx context.Context, topic string, result []byte) (ResearchSession, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" || len(result) == 0 {
		return ResearchSession{}, ErrInvalidState
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ResearchSession{}, err
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx, `
SELECT id FROM research_sessions
WHERE lower(topic) = lower(?)
ORDER BY created_at DESC, id DESC
LIMIT 1`, topic).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResearchSession{}, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE research_sessions
SET topic = ?, result = ?, created_at = CURRENT_TIMESTAMP
WHERE id = ?`, topic, string(result), existingID); err != nil {
			return ResearchSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM research_sessions
WHERE lower(topic) = lower(?) AND id != ?`, topic, existingID); err != nil {
			return ResearchSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return ResearchSession{}, err
		}
		return s.GetResearchSession(ctx, existingID)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO research_sessions (topic, result)
VALUES (?, ?)`, topic, string(result))
	if err != nil {
		return ResearchSession{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ResearchSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResearchSession{}, err
	}
	return s.GetResearchSession(ctx, id)
}

func (s *Store) GetResearchSession(ctx context.Context, id int64) (ResearchSession, error) {
	var session ResearchSession
	err := s.db.QueryRowContext(ctx, `
SELECT id, topic, result, created_at
FROM research_sessions
WHERE id = ?`, id).Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ResearchSession{}, ErrNotFound
	}
	if err != nil {
		return ResearchSession{}, err
	}
	return session, nil
}

func (s *Store) ListResearchSessions(ctx context.Context, limit int) ([]ResearchSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, topic, result, created_at
FROM research_sessions
ORDER BY created_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ResearchSession
	for rows.Next() {
		var session ResearchSession
		if err := rows.Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) DeleteResearchSession(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM research_sessions WHERE id = ?`, id)
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

// ---- Recommendation Sessions ----

func (s *Store) CreateRecommendationSession(ctx context.Context, topic string, result []byte) (RecommendationSession, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" || len(result) == 0 {
		return RecommendationSession{}, ErrInvalidState
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RecommendationSession{}, err
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx, `
SELECT id FROM recommendation_sessions
WHERE lower(topic) = lower(?)
ORDER BY created_at DESC, id DESC
LIMIT 1`, topic).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return RecommendationSession{}, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE recommendation_sessions
SET topic = ?, result = ?, created_at = CURRENT_TIMESTAMP
WHERE id = ?`, topic, string(result), existingID); err != nil {
			return RecommendationSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM recommendation_sessions
WHERE lower(topic) = lower(?) AND id != ?`, topic, existingID); err != nil {
			return RecommendationSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return RecommendationSession{}, err
		}
		return s.GetRecommendationSession(ctx, existingID)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO recommendation_sessions (topic, result)
VALUES (?, ?)`, topic, string(result))
	if err != nil {
		return RecommendationSession{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RecommendationSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecommendationSession{}, err
	}
	return s.GetRecommendationSession(ctx, id)
}

func (s *Store) GetRecommendationSession(ctx context.Context, id int64) (RecommendationSession, error) {
	var session RecommendationSession
	err := s.db.QueryRowContext(ctx, `
SELECT id, topic, result, created_at
FROM recommendation_sessions
WHERE id = ?`, id).Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RecommendationSession{}, ErrNotFound
	}
	if err != nil {
		return RecommendationSession{}, err
	}
	return session, nil
}

func (s *Store) ListRecommendationSessions(ctx context.Context, limit int) ([]RecommendationSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, topic, result, created_at
FROM recommendation_sessions
ORDER BY created_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecommendationSession
	for rows.Next() {
		var session RecommendationSession
		if err := rows.Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRecommendationSession(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM recommendation_sessions WHERE id = ?`, id)
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
