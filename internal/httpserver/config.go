package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/colinleefish/rmb-desktop/internal/config"
	"github.com/colinleefish/rmb-desktop/internal/launchatlogin"
	"github.com/colinleefish/rmb-desktop/internal/reembed"
	"github.com/colinleefish/rmb-desktop/internal/llm"
)

type configTestSide struct {
	OK             bool     `json:"ok"`
	LatencyMs      int64    `json:"latency_ms,omitempty"`
	Error          string   `json:"error,omitempty"`
	RequestedModel string   `json:"requested_model,omitempty"`
	ModelFound     *bool    `json:"model_found,omitempty"`
	ModelsCount    int      `json:"models_count,omitempty"`
	Models         []string `json:"models,omitempty"`
}

type configTestResponse struct {
	LLM   configTestSide `json:"llm"`
	Embed configTestSide `json:"embed"`
}

type configTestRequest struct {
	LLM struct {
		APIBase string `json:"api_base"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	} `json:"llm"`
	Embed struct {
		APIBase    string `json:"api_base"`
		APIKey     string `json:"api_key"`
		Model      string `json:"model"`
		Dimensions int    `json:"dimensions"`
	} `json:"embed"`
}

func (s *Server) handlePostConfigTest(w http.ResponseWriter, r *http.Request) {
	var req configTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	out := configTestResponse{}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		llmRes, err := llm.TestLLMConnection(ctx, config.LLMConfig{
			APIBase: req.LLM.APIBase,
			APIKey:  req.LLM.APIKey,
			Model:   req.LLM.Model,
		})
		out.LLM = llmTestSide(llmRes, err)
	}()

	go func() {
		defer wg.Done()
		embedDur, err := llm.TestEmbedConnection(ctx, config.EmbedConfig{
			APIBase:    req.Embed.APIBase,
			APIKey:     req.Embed.APIKey,
			Model:      req.Embed.Model,
			Dimensions: req.Embed.Dimensions,
		})
		if err != nil {
			out.Embed.Error = err.Error()
		} else {
			out.Embed.OK = true
			out.Embed.LatencyMs = embedDur.Milliseconds()
		}
	}()

	wg.Wait()
	writeJSON(w, http.StatusOK, out)
}

type configTestLLMBody struct {
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type configTestEmbedBody struct {
	APIBase    string `json:"api_base"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

func writeConfigTestSide(w http.ResponseWriter, dur time.Duration, err error) {
	out := configTestSide{}
	if err != nil {
		out.Error = err.Error()
	} else {
		out.OK = true
		out.LatencyMs = dur.Milliseconds()
	}
	writeJSON(w, http.StatusOK, out)
}

func llmTestSide(res llm.LLMConnectionTestResult, err error) configTestSide {
	out := configTestSide{
		LatencyMs:      res.Latency.Milliseconds(),
		RequestedModel: res.RequestedModel,
		Models:         res.Models,
	}
	if res.RequestedModel != "" {
		found := res.ModelFound
		out.ModelFound = &found
	}
	if res.ModelsTotal > 0 {
		out.ModelsCount = res.ModelsTotal
	}
	if len(res.Models) > 0 {
		out.Models = res.Models
	}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.OK = true
	return out
}

func (s *Server) handlePostConfigTestLLM(w http.ResponseWriter, r *http.Request) {
	var req configTestLLMBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	res, err := llm.TestLLMConnection(ctx, config.LLMConfig{
		APIBase: req.APIBase,
		APIKey:  req.APIKey,
		Model:   req.Model,
	})
	writeJSON(w, http.StatusOK, llmTestSide(res, err))
}

func (s *Server) handlePostConfigTestEmbed(w http.ResponseWriter, r *http.Request) {
	var req configTestEmbedBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	dur, err := llm.TestEmbedConnection(ctx, config.EmbedConfig{
		APIBase:    req.APIBase,
		APIKey:     req.APIKey,
		Model:      req.Model,
		Dimensions: req.Dimensions,
	})
	writeConfigTestSide(w, dur, err)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, config.ToView(cfg, s.configPath))
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var req config.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	embedBefore := cfg.Embed
	updated, err := config.ApplyUpdate(cfg, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	reembedNeeded := reembed.SettingsChanged(embedBefore, updated.Embed)
	if reembedNeeded {
		if err := reembed.ClearAll(r.Context(), s.db); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.log.Info("embed settings changed; cleared stored vectors for re-embedding")
	}
	if err := launchatlogin.Set(updated.LaunchAtLogin); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := config.Save(s.configPath, updated); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("config saved", "path", s.configPath)
	message := "Config saved. Restart rmbd to apply LLM/embed worker changes."
	if reembedNeeded {
		message = "Config saved. Embedding settings changed — vectors cleared; restart rmbd to re-embed all memories."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"message":         message,
		"reembed_started": reembedNeeded,
		"config":          config.ToView(updated, s.configPath),
	})
}
