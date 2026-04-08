package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

func TestHandleInstallBatch_AgentInstallWritesMetadataToAgentsSource(t *testing.T) {
	s, skillsDir := newTestServer(t)

	agentsDir := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	s.cfg.AgentsSource = agentsDir
	s.agentsStore = install.NewMetadataStore()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	agentPath := filepath.Join(repoDir, "reviewer.md")
	if err := os.WriteFile(agentPath, []byte("# Reviewer agent"), 0644); err != nil {
		t.Fatalf("failed to write agent file: %v", err)
	}
	for _, args := range [][]string{
		{"add", "reviewer.md"},
		{"commit", "-m", "add reviewer agent"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	payload, err := json.Marshal(map[string]any{
		"source": "file://" + repoDir,
		"skills": []map[string]string{
			{"name": "reviewer", "path": "reviewer.md"},
		},
		"kind": "agent",
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/install/batch", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, body=%s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(agentsDir, "reviewer.md")); err != nil {
		t.Fatalf("expected installed agent in agents source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsDir, install.MetadataFileName)); err != nil {
		t.Fatalf("expected metadata written to agents source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, install.MetadataFileName)); !os.IsNotExist(err) {
		t.Fatalf("expected no agent metadata written to skills source, got err=%v", err)
	}
}
