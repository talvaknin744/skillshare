//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// --- update agents ---

func TestUpdate_Agents_NoAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, nil)
	_ = agentsDir

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No agents found")
}

func TestUpdate_Agents_LocalOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "local")
}

func TestUpdate_Agents_GroupInvalidDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--group", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUpdate_Agents_RequiresNameOrAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "specify agent name")
}

// --- uninstall agents ---

func TestUninstall_Agents_RemovesToTrash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Removed agent")
	result.AssertAnyOutputContains(t, "tutor")

	// Verify agent file was removed from source
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("agent file should be removed from source")
	}
}

func TestUninstall_Agents_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "nonexistent", "--force")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUninstall_Agents_All(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "--all", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "2 agent(s) removed")

	// Verify both files removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("tutor.md should be removed")
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "reviewer.md")); !os.IsNotExist(err) {
		t.Error("reviewer.md should be removed")
	}
}

// --- collect agents ---

func TestCollect_Agents_NoLocalAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	// Sync agents first (creates symlinks)
	sb.RunCLI("sync", "agents")

	// Collect should find no local (non-symlinked) agents
	result := sb.RunCLI("collect", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No local agents")
}

func TestCollect_Agents_CollectsLocalFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")

	// Create a local (non-symlinked) agent in the target
	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local agent"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	agentsSource := filepath.Join(filepath.Dir(sb.SourcePath), "agents")

	result := sb.RunCLI("collect", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "collected")

	// Verify the file was copied to agent source
	if _, err := os.Stat(filepath.Join(agentsSource, "local-agent.md")); err != nil {
		t.Error("local-agent.md should be collected to agent source")
	}
}

// --- trash agents ---

func TestTrash_Agents_ListEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("trash", "agents", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")
}

func TestTrash_Agents_ListAfterUninstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall to trash
	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// List agent trash
	result := sb.RunCLI("trash", "agents", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
}

func TestTrash_Agents_Restore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall
	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// Verify removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Fatal("should be removed after uninstall")
	}

	// Restore
	result := sb.RunCLI("trash", "agents", "restore", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Restored")

	// Verify restored to agent source
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); err != nil {
		t.Error("tutor.md should be restored to agent source")
	}
}

// --- default behavior unchanged ---

func TestTrash_Default_SkillsOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Default trash list should check skill trash (not agent trash)
	result := sb.RunCLI("trash", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")
}
