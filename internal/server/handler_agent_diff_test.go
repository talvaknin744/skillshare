package server

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/resource"
)

func TestComputeAgentTargetDiff_MissingInTarget(t *testing.T) {
	targetDir := t.TempDir()

	agents := []resource.DiscoveredResource{
		{FlatName: "helper.md", AbsPath: "/src/helper.md", RelPath: "helper.md"},
	}

	items := computeAgentTargetDiff(targetDir, agents)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "link" {
		t.Errorf("expected action 'link', got %q", items[0].Action)
	}
	if items[0].Kind != "agent" {
		t.Errorf("expected kind 'agent', got %q", items[0].Kind)
	}
}

func TestComputeAgentTargetDiff_OrphanSymlink(t *testing.T) {
	targetDir := t.TempDir()
	os.Symlink("/nonexistent/old.md", filepath.Join(targetDir, "orphan.md"))

	items := computeAgentTargetDiff(targetDir, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "prune" {
		t.Errorf("expected action 'prune', got %q", items[0].Action)
	}
}

func TestComputeAgentTargetDiff_LocalFile(t *testing.T) {
	targetDir := t.TempDir()
	os.WriteFile(filepath.Join(targetDir, "local.md"), []byte("# Local"), 0644)

	items := computeAgentTargetDiff(targetDir, nil)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Action != "local" {
		t.Errorf("expected action 'local', got %q", items[0].Action)
	}
}

func TestComputeAgentTargetDiff_InSync(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	srcFile := filepath.Join(sourceDir, "agent.md")
	os.WriteFile(srcFile, []byte("# Agent"), 0644)
	os.Symlink(srcFile, filepath.Join(targetDir, "agent.md"))

	agents := []resource.DiscoveredResource{
		{FlatName: "agent.md", AbsPath: srcFile, RelPath: "agent.md"},
	}

	items := computeAgentTargetDiff(targetDir, agents)

	if len(items) != 0 {
		t.Fatalf("expected 0 items (in sync), got %d", len(items))
	}
}
