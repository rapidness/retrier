package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/your-org/proxy-api/internal/config"
	"github.com/your-org/proxy-api/internal/proxy"
)

// Handlers contains the REST API handlers.
type Handlers struct {
	cfgMgr  *config.ConfigManager
	handler *proxy.Handler
}

// NewHandlers creates API handlers.
func NewHandlers(cfgMgr *config.ConfigManager, handler *proxy.Handler) *Handlers {
	return &Handlers{cfgMgr: cfgMgr, handler: handler}
}

// RegisterRoutes registers all API routes on the given mux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/config", h.handleConfig)
	mux.HandleFunc("/api/rules", h.handleRules)
	mux.HandleFunc("/api/rules/", h.handleRuleByName)
	mux.HandleFunc("/api/logging", h.handleLogging)
	mux.HandleFunc("/api/status", h.handleStatus)
}

// GET /api/config — return current config
// PUT /api/config — update entire config
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.cfgMgr.Snapshot()
		writeJSON(w, http.StatusOK, cfg)

	case http.MethodPut:
		var cfg config.Config
		if err := readJSON(r, &cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.cfgMgr.SaveAndReload(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.handler.UpdateConfig(h.cfgMgr.Snapshot())
		writeJSON(w, http.StatusOK, h.cfgMgr.Snapshot())

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GET /api/rules — list rules
// POST /api/rules — add rule
func (h *Handlers) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.cfgMgr.Snapshot()
		writeJSON(w, http.StatusOK, cfg.Rules)

	case http.MethodPost:
		var rule config.Rule
		if err := readJSON(r, &rule); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		cfg := *h.cfgMgr.Snapshot() // shallow copy
		cfg.Rules = append(cfg.Rules, rule)
		if err := h.cfgMgr.SaveAndReload(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.handler.UpdateConfig(h.cfgMgr.Snapshot())
		writeJSON(w, http.StatusOK, rule)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// GET/PUT/DELETE /api/rules/:name
func (h *Handlers) handleRuleByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/rules/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "rule name required")
		return
	}

	cfg := *h.cfgMgr.Snapshot()

	switch r.Method {
	case http.MethodPut:
		var rule config.Rule
		if err := readJSON(r, &rule); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		found := false
		for i, r := range cfg.Rules {
			if r.Name == name {
				cfg.Rules[i] = rule
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if err := h.cfgMgr.SaveAndReload(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.handler.UpdateConfig(h.cfgMgr.Snapshot())
		writeJSON(w, http.StatusOK, rule)

	case http.MethodDelete:
		found := false
		for i, r := range cfg.Rules {
			if r.Name == name {
				cfg.Rules = append(cfg.Rules[:i], cfg.Rules[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if err := h.cfgMgr.SaveAndReload(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		h.handler.UpdateConfig(h.cfgMgr.Snapshot())
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// PUT /api/logging — update logging config
func (h *Handlers) handleLogging(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var logging config.LoggingConfig
	if err := readJSON(r, &logging); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg := *h.cfgMgr.Snapshot()
	cfg.Logging = logging
	if err := h.cfgMgr.SaveAndReload(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.handler.UpdateConfig(h.cfgMgr.Snapshot())
	writeJSON(w, http.StatusOK, h.cfgMgr.Snapshot().Logging)
}

// GET /api/status — runtime status
func (h *Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfgMgr.Snapshot()
	status := map[string]interface{}{
		"proxy_listening": cfg.Proxy.Listen,
		"upstream":        cfg.Proxy.Upstream,
		"rules_count":     len(cfg.Rules),
		"logging_enabled": cfg.Logging.Enabled,
		"retry_total":     0,
		"retry_success":   0,
		"retry_exhausted": 0,
	}
	writeJSON(w, http.StatusOK, status)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}
