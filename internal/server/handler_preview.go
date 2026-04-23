package server

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	ghclient "skillshare/internal/github"
	"skillshare/internal/search"
)

var (
	previewCache    sync.Map
	previewCacheTTL = 5 * time.Minute
)

type previewCacheEntry struct {
	data      *search.SkillPreview
	expiresAt time.Time
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing required parameter: source")
		return
	}
	branch := r.URL.Query().Get("branch")

	// Parse owner/repo/path from source string
	parts := strings.SplitN(source, "/", 3)
	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid source format: expected owner/repo[/path]")
		return
	}
	owner := parts[0]
	repo := parts[1]
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid source: owner and repo must not be empty")
		return
	}
	path := ""
	if len(parts) == 3 {
		path = parts[2]
	}

	// Check cache (include branch in key to avoid cross-branch collisions)
	cacheKey := source + "@" + branch
	if entry, ok := previewCache.Load(cacheKey); ok {
		ce := entry.(*previewCacheEntry)
		if time.Now().Before(ce.expiresAt) {
			writeJSON(w, ce.data)
			return
		}
		previewCache.Delete(cacheKey)
	}

	client := ghclient.NewClient()
	preview, err := search.FetchSkillContent(client, owner, repo, path, branch)
	if err != nil {
		var rlErr *ghclient.RateLimitError
		if errors.As(err, &rlErr) {
			w.Header().Set("Retry-After", rlErr.Reset)
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Store in cache
	previewCache.Store(cacheKey, &previewCacheEntry{
		data:      preview,
		expiresAt: time.Now().Add(previewCacheTTL),
	})

	writeJSON(w, preview)
}
