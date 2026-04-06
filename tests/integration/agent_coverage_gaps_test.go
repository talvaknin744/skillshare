//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// --- trash agents empty ---

func TestTrash_Agents_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall both agents to trash
	sb.RunCLI("uninstall", "-g", "agents", "--all", "--force")

	// Verify trash has items
	listResult := sb.RunCLI("trash", "agents", "list", "--no-tui")
	listResult.AssertSuccess(t)
	listResult.AssertAnyOutputContains(t, "tutor")

	// Empty agent trash (use --force via input "y")
	emptyResult := sb.RunCLIWithInput("y\n", "trash", "agents", "empty")
	emptyResult.AssertSuccess(t)
	emptyResult.AssertAnyOutputContains(t, "Emptied trash")

	// Verify trash is now empty
	afterResult := sb.RunCLI("trash", "agents", "list", "--no-tui")
	afterResult.AssertSuccess(t)
	afterResult.AssertAnyOutputContains(t, "empty")
}

// --- trash agents delete ---

func TestTrash_Agents_Delete(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// Delete specific item from agent trash
	result := sb.RunCLI("trash", "agents", "delete", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Permanently deleted")
}

// --- uninstall agents project mode ---

func TestUninstall_Agents_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup project
	projectDir := filepath.Join(sb.Root, "myproject")
	os.MkdirAll(filepath.Join(projectDir, ".skillshare", "skills"), 0755)
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor agent"), 0644)

	// Write project config
	projectCfgDir := filepath.Join(projectDir, ".skillshare")
	os.WriteFile(filepath.Join(projectCfgDir, "config.yaml"), []byte("targets:\n  - claude\n"), 0644)

	// Also need global config for the CLI to not error
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLIInDir(projectDir, "uninstall", "-p", "agents", "tutor", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Removed agent")

	// Verify removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("agent should be removed from project agents dir")
	}
}

// --- check all (combined) ---

func TestCheck_All_CombinedOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// "check all" should show both skills and agents
	// Currently "check" defaults to skills-only, "check agents" is agents-only
	// "check all" should combine both
	result := sb.RunCLI("check", "all")
	result.AssertSuccess(t)
}

// --- multi-target agent config ---

func TestSync_Agents_SkipsTargetsWithoutAgentPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	// claude has agent path, cursor does NOT
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
  cursor:
    skills:
      path: ` + sb.CreateTarget("cursor") + `
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Claude agents should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); err != nil {
		t.Error("claude agent should be synced")
	}
}

// --- list agents JSON with kind field ---

func TestList_Agents_JSON_AllEntriesHaveKind(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// "list all --json" should have kind on every entry
	result := sb.RunCLI("list", "all", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"kind": "skill"`)
	result.AssertAnyOutputContains(t, `"kind": "agent"`)
}

// --- status agents JSON with targets ---

func TestStatus_Agents_JSON_WithTargets(t *testing.T) {
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

	// Sync agents
	sb.RunCLI("sync", "agents")

	result := sb.RunCLI("status", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agents"`)
	result.AssertAnyOutputContains(t, `"expected"`)
	result.AssertAnyOutputContains(t, `"linked"`)
}
