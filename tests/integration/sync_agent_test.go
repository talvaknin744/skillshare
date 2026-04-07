//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_Agents_SkipsTargetsWithoutAgentsPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create agents source with an agent
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper"), 0644)

	// Configure a target WITH agents path and one WITHOUT
	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	windsurf := filepath.Join(sb.Home, ".windsurf", "skills")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
  windsurf:
    skills:
      path: "` + windsurf + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Agent should be synced to claude
	if !sb.FileExists(filepath.Join(claudeAgents, "helper.md")) {
		t.Error("agent should be synced to claude agents dir")
	}

	// Warning should mention windsurf was skipped
	result.AssertAnyOutputContains(t, "skipped")
	result.AssertAnyOutputContains(t, "windsurf")
}
