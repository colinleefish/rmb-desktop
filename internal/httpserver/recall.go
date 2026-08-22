package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/inspect"
	"github.com/colinleefish/rmb-desktop/internal/recall"
)

// lsOptionsFromQuery maps the optional ls query parameters (limit, offset,
// since, until, count) onto inspect.LsOptions. Unknown or malformed values
// are rejected with a descriptive error.
func lsOptionsFromQuery(r *http.Request) (inspect.LsOptions, error) {
	opts := inspect.DefaultLsOptions()
	q := r.URL.Query()
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return opts, fmt.Errorf("limit must be a non-negative integer")
		}
		opts.Limit = n
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return opts, fmt.Errorf("offset must be a non-negative integer")
		}
		opts.Offset = n
	}
	if v := strings.TrimSpace(q.Get("since")); v != "" {
		ts, err := inspect.ParseTimeFilter(v, time.Now())
		if err != nil {
			return opts, err
		}
		opts.Since = ts
	}
	if v := strings.TrimSpace(q.Get("until")); v != "" {
		ts, err := inspect.ParseTimeFilter(v, time.Now())
		if err != nil {
			return opts, err
		}
		opts.Until = ts
	}
	if v := strings.TrimSpace(q.Get("count")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return opts, fmt.Errorf("count must be true or false")
		}
		opts.Count = b
	}
	return opts, nil
}

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

	var tw recall.TimeWindow
	now := time.Now()
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		ms, err := recall.ParseTimeValue(raw, now)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since: "+err.Error())
			return
		}
		tw.SinceMS = ms
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("until")); raw != "" {
		ms, err := recall.ParseTimeValue(raw, now)
		if err != nil {
			writeError(w, http.StatusBadRequest, "until: "+err.Error())
			return
		}
		tw.UntilMS = ms
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

	var opts []recall.SearchOption
	if nb := strings.TrimSpace(r.URL.Query().Get("no_boost")); nb != "" && nb != "0" && nb != "false" {
		opts = append(opts, recall.WithNoBoost())
	}

	matches, err := s.recall.Search(r.Context(), embedder, query, k, scopes, tw, opts...)
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
		// Per-URI exposure counter (raw signal; heat is NOT updated here).
		s.recallStats.RecordSearchAsync(uris)
		// Local-only query log for the search→cat join and doctor metrics.
		s.recallStats.RecordQueryAsync(query, scopes, k, uris)
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
		opts, perr := lsOptionsFromQuery(r)
		if perr != nil {
			writeError(w, http.StatusBadRequest, perr.Error())
			return
		}
		err = s.inspect.LsWith(r.Context(), rawURI, opts, w)
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
