package httpserver

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/colinleefish/rmb-desktop/internal/recall"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}

	k := 5
	if v := strings.TrimSpace(r.URL.Query().Get("k")); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "k must be a positive integer")
			return
		}
		k = parsed
	}

	var scopes []string
	if raw := strings.TrimSpace(r.URL.Query().Get("scope")); raw != "" {
		for _, sc := range strings.Split(raw, ",") {
			sc = strings.TrimSpace(sc)
			if sc == "" {
				continue
			}
			scopes = append(scopes, sc)
		}
	}

	var embedder recall.QueryEmbedder
	if s.embed != nil {
		embedder = func(ctx context.Context, q string) ([]float32, error) {
			vecs, err := s.embed.Embed(ctx, []string{q})
			if err != nil {
				return nil, err
			}
			if len(vecs) == 0 {
				return nil, nil
			}
			return vecs[0], nil
		}
	}

	matches, err := s.recall.Search(r.Context(), embedder, query, k, scopes)
	if err != nil {
		s.log.Error("search failed", "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.recallStats != nil && len(matches) > 0 {
		uris := make([]string, len(matches))
		for i, m := range matches {
			uris[i] = m.URI
		}
		s.recallStats.RecordSearchAsync(uris)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": matches})
}

func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request, kind string) {
	rawURI := strings.TrimSpace(r.URL.Query().Get("uri"))
	if rawURI == "" {
		writeError(w, http.StatusBadRequest, "uri is required")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	var err error
	switch kind {
	case "cat":
		err = s.inspect.Cat(r.Context(), rawURI, w)
	case "ls":
		err = s.inspect.Ls(r.Context(), rawURI, w)
	case "meta":
		err = s.inspect.Meta(r.Context(), rawURI, w)
	default:
		writeError(w, http.StatusNotFound, "unknown inspect kind")
		return
	}
	if err != nil {
		s.log.Error("inspect failed", "kind", kind, "uri", rawURI, "err", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.recallStats != nil {
		switch kind {
		case "cat":
			s.recallStats.RecordCatAsync(rawURI)
		case "meta":
			s.recallStats.RecordMetaAsync(rawURI)
		}
	}
}
