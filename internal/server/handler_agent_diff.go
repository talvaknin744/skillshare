package server

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/resource"
	"skillshare/internal/utils"
)

// computeAgentTargetDiff computes diff items for agents in a single target directory.
// Returns items with Kind="agent" for each pending action (link, update, prune, local).
func computeAgentTargetDiff(targetDir string, agents []resource.DiscoveredResource) []diffItem {
	var items []diffItem

	// Build expected set
	expected := make(map[string]resource.DiscoveredResource, len(agents))
	for _, a := range agents {
		expected[a.FlatName] = a
	}

	// Read existing .md files in target
	existing := make(map[string]os.FileMode)
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			if resource.ConventionalExcludes[e.Name()] {
				continue
			}
			existing[e.Name()] = e.Type()
		}
	}

	// Missing agents → link
	for flatName, agent := range expected {
		if _, ok := existing[flatName]; !ok {
			items = append(items, diffItem{
				Skill:  flatName,
				Action: "link",
				Reason: "source only",
				Kind:   "agent",
			})
			continue
		}
		// Exists — check if symlink points to correct source
		targetPath := filepath.Join(targetDir, flatName)
		if utils.IsSymlinkOrJunction(targetPath) {
			absLink, err := utils.ResolveLinkTarget(targetPath)
			if err != nil {
				items = append(items, diffItem{
					Skill:  flatName,
					Action: "update",
					Reason: "link target unreadable",
					Kind:   "agent",
				})
				continue
			}
			absSource, _ := filepath.Abs(agent.AbsPath)
			if !utils.PathsEqual(absLink, absSource) {
				items = append(items, diffItem{
					Skill:  flatName,
					Action: "update",
					Reason: "symlink points elsewhere",
					Kind:   "agent",
				})
			}
			// else: in sync, no item emitted
		}
		// Non-symlink existing file: already local, no action needed for expected agents
	}

	// Orphan/local detection
	for name, fileType := range existing {
		if _, ok := expected[name]; ok {
			continue
		}
		if fileType&os.ModeSymlink != 0 {
			items = append(items, diffItem{
				Skill:  name,
				Action: "prune",
				Reason: "orphan symlink",
				Kind:   "agent",
			})
		} else {
			items = append(items, diffItem{
				Skill:  name,
				Action: "local",
				Reason: "local file",
				Kind:   "agent",
			})
		}
	}

	return items
}
