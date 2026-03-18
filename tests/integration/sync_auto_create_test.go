//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_AutoCreatesTargetDir_WhenParentExists(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	// Create parent directory (~/.claude) but NOT the skills subdirectory
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	parentPath := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created target directory:")

	// Verify skills directory was created and skill was synced
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("target directory should have been created")
	}
	skillLink := filepath.Join(targetPath, "my-skill")
	if !sb.IsSymlink(skillLink) {
		t.Error("skill should be a symlink after auto-create")
	}
}

func TestSync_AutoCreatesTargetDir_WhenParentAlsoMissing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	// Target path where parent also doesn't exist (e.g., universal target ~/.agents/skills)
	targetPath := filepath.Join(sb.Home, ".agents", "skills")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  universal:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created target directory:")

	// Verify directory was created
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("target directory should have been created even with missing parent")
	}
}

func TestSync_AutoCreatesTargetDir_CopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	// Create parent but not skills dir
	targetPath := filepath.Join(sb.Home, ".cursor", "skills")
	parentPath := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  cursor:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created target directory:")

	// Verify skill was copied
	skillDir := filepath.Join(targetPath, "my-skill")
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		t.Error("skill should exist after auto-create + copy")
	}
}

func TestSync_AutoCreateTargetDir_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	// Create parent but not skills dir
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	parentPath := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		t.Fatalf("failed to create parent: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "--dry-run")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Created target directory:")

	// Verify directory was NOT actually created
	if _, err := os.Stat(targetPath); err == nil {
		t.Error("target directory should not be created during dry-run")
	}
}
