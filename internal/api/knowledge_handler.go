package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"note/internal/store"
)

func (s *Server) handleListKnowledgeCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Status:          strings.TrimSpace(r.URL.Query().Get("status")),
		IncludeArchived: parseBool(r.URL.Query().Get("include_archived")),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if cards == nil {
		cards = []store.KnowledgeCard{}
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleDueKnowledgeCards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	cards, err := s.store.ListKnowledgeCards(ctx, store.CardFilter{DueOnly: true})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if cards == nil {
		cards = []store.KnowledgeCard{}
	}
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleGetKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.GetKnowledgeCard(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleCreateKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	var req store.KnowledgeCardInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.CreateKnowledgeCard(ctx, req)
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "front and back are required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (s *Server) handleUpdateKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req store.KnowledgeCardInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.UpdateKnowledgeCard(ctx, id, req)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "front and back are required")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (s *Server) handleDeleteKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err := s.store.DeleteKnowledgeCard(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleReviewKnowledgeCard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Remembered bool   `json:"remembered"`
		Action     string `json:"action"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	remembered := req.Remembered || strings.EqualFold(strings.TrimSpace(req.Action), "remembered")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	card, err := s.store.ReviewKnowledgeCard(ctx, id, remembered, time.Now())
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, card)
}
