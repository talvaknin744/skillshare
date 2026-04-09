package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/check"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
)

// cmdUpdateAgents handles "skillshare update agents [name|--all]".
func cmdUpdateAgents(args []string, cfg *config.Config, start time.Time) error {
	opts, showHelp, parseErr := parseUpdateAgentArgs(args)
	if showHelp {
		printUpdateHelp()
		return nil
	}
	if parseErr != nil {
		return parseErr
	}

	agentsDir := cfg.EffectiveAgentsSource()
	if _, err := os.Stat(agentsDir); err != nil {
		if os.IsNotExist(err) {
			ui.Info("No agents source directory (%s)", agentsDir)
			return nil
		}
		return fmt.Errorf("cannot access agents source: %w", err)
	}

	// Discover agents and check status
	results := check.CheckAgents(agentsDir)
	if len(results) == 0 {
		ui.Info("No agents found")
		return nil
	}

	// Filter by name if specified
	if len(opts.names) > 0 {
		results = filterAgentCheckResults(results, opts.names)
		if len(results) == 0 {
			return fmt.Errorf("no matching agents found: %s", strings.Join(opts.names, ", "))
		}
	}

	// Filter by group if specified
	if len(opts.groups) > 0 {
		var err error
		results, err = filterAgentResultsByGroups(results, opts.groups, agentsDir)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return fmt.Errorf("no agents found in group(s): %s", strings.Join(opts.groups, ", "))
		}
	}

	// Only check agents that have remote sources
	tracked := collectTrackedAgentResults(results)

	if len(tracked) == 0 {
		ui.Info("No tracked agents to update (all are local)")
		return nil
	}

	// Enrich with remote status
	if !opts.jsonOutput {
		sp := ui.StartSpinner(fmt.Sprintf("Checking %d agent(s) for updates...", len(tracked)))
		check.EnrichAgentResultsWithRemote(tracked, func() { sp.Success("Check complete") })
	} else {
		check.EnrichAgentResultsWithRemote(tracked, nil)
	}

	// Find agents with updates available
	var updatable []check.AgentCheckResult
	for _, r := range tracked {
		if r.Status == "update_available" {
			updatable = append(updatable, r)
		}
	}

	if len(updatable) == 0 {
		if !opts.jsonOutput {
			ui.Success("All agents are up to date")
		}
		if opts.jsonOutput {
			return updateAgentsOutputJSON(nil, opts.dryRun, start, nil)
		}
		return nil
	}

	if !opts.jsonOutput {
		ui.Header("Updating agents")
		if opts.dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	// Update agents, batching by repo URL to share git clones.
	var updated, failed int
	if opts.dryRun {
		for _, r := range updatable {
			if !opts.jsonOutput {
				ui.Info("  %s: update available from %s", r.Name, r.Source)
			}
		}
	} else {
		updated, failed = batchUpdateAgents(agentsDir, updatable, !opts.jsonOutput)
	}

	if !opts.jsonOutput && !opts.dryRun {
		fmt.Println()
		ui.Info("Agent update: %d updated, %d failed", updated, failed)
	}

	logUpdateAgentOp(config.ConfigPath(), len(updatable), updated, failed, opts.dryRun, start)

	if opts.jsonOutput {
		return updateAgentsOutputJSON(updatable, opts.dryRun, start, nil)
	}

	if failed > 0 {
		return fmt.Errorf("%d agent(s) failed to update", failed)
	}
	return nil
}

// agentRepoKey groups agents by clone URL + branch + repo subdir so agents
// from the same scope share a single git clone.
type agentRepoKey struct {
	cloneURL   string
	branch     string
	repoSubdir string
}

// batchUpdateAgents groups agents by repo URL and clones once per group.
// Agents with no RepoURL fall back to per-agent reinstallAgent.
func batchUpdateAgents(agentsDir string, agents []check.AgentCheckResult, verbose bool) (updated, failed int) {
	store := install.LoadMetadataOrNew(agentsDir)
	trackedRepos := map[string][]check.AgentCheckResult{}
	groups := map[agentRepoKey][]check.AgentCheckResult{}
	var noRepo []check.AgentCheckResult

	for _, r := range agents {
		if r.RepoPath != "" {
			trackedRepos[r.RepoPath] = append(trackedRepos[r.RepoPath], r)
			continue
		}
		if r.RepoURL == "" {
			noRepo = append(noRepo, r)
			continue
		}
		entry := store.GetByPath(r.Name)
		if entry == nil || entry.Source == "" {
			noRepo = append(noRepo, r)
			continue
		}

		source, parseErr := install.ParseSource(entry.Source)
		if parseErr != nil {
			noRepo = append(noRepo, r)
			continue
		}
		repoSubdir := strings.TrimSuffix(source.Subdir, entry.Subdir)
		repoSubdir = strings.TrimRight(repoSubdir, "/")

		key := agentRepoKey{
			cloneURL:   r.RepoURL,
			branch:     entry.Branch,
			repoSubdir: repoSubdir,
		}
		groups[key] = append(groups[key], r)
	}

	for repoPath, members := range trackedRepos {
		uc := &updateContext{
			sourcePath: agentsDir,
			opts:       &updateOptions{},
		}
		ok, _, err := updateTrackedRepoQuick(uc, repoPath)
		if err != nil {
			for _, m := range members {
				if verbose {
					ui.Error("  %s: %v", m.Name, err)
				}
				failed++
			}
			continue
		}
		if !ok {
			continue
		}
		for _, m := range members {
			if verbose {
				ui.Success("  %s: updated", m.Name)
			}
			updated++
		}
	}

	// Batch: one clone per repo group
	for key, members := range groups {
		source := &install.Source{
			CloneURL: key.cloneURL,
			Subdir:   key.repoSubdir,
			Branch:   key.branch,
		}

		var discovery *install.DiscoveryResult
		var discErr error
		if source.HasSubdir() {
			discovery, discErr = install.DiscoverFromGitSubdir(source)
		} else {
			discovery, discErr = install.DiscoverFromGit(source)
		}
		if discErr != nil {
			for _, m := range members {
				if verbose {
					ui.Error("  %s: discovery failed: %v", m.Name, discErr)
				}
				failed++
			}
			continue
		}

		// Build agent name → AgentInfo lookup
		agentIndex := map[string]*install.AgentInfo{}
		for i, a := range discovery.Agents {
			agentIndex[a.Name] = &discovery.Agents[i]
		}

		for _, m := range members {
			agentName := filepath.Base(m.Name)
			target := agentIndex[agentName]
			if target == nil {
				if verbose {
					ui.Error("  %s: not found in repository", m.Name)
				}
				failed++
				continue
			}

			destDir := agentsDir
			if dir := filepath.Dir(m.Name); dir != "." {
				destDir = filepath.Join(agentsDir, dir)
			}

			opts := install.InstallOptions{Kind: "agent", Force: true, SourceDir: agentsDir}
			if _, err := install.InstallAgentFromDiscovery(discovery, *target, destDir, opts); err != nil {
				if verbose {
					ui.Error("  %s: %v", m.Name, err)
				}
				failed++
			} else {
				if verbose {
					ui.Success("  %s: updated", m.Name)
				}
				updated++
			}
		}

		install.CleanupDiscovery(discovery)
	}

	// Fallback: agents without RepoURL
	for _, r := range noRepo {
		if err := reinstallAgent(agentsDir, r, store); err != nil {
			if verbose {
				ui.Error("  %s: %v", r.Name, err)
			}
			failed++
		} else {
			if verbose {
				ui.Success("  %s: updated", r.Name)
			}
			updated++
		}
	}

	return updated, failed
}

// reinstallAgent re-installs an agent from its recorded source using
// discovery + InstallAgentFromDiscovery (single-file copy), not the
// directory-based skill installer.
// Used as fallback for agents without RepoURL in the batch path.
func reinstallAgent(agentsDir string, r check.AgentCheckResult, store *install.MetadataStore) error {
	entry := store.GetByPath(r.Name)
	if entry == nil || entry.Source == "" {
		return fmt.Errorf("no source metadata for agent %q", r.Name)
	}

	// Reconstruct the repo-level subdir for discovery.
	source, parseErr := install.ParseSource(entry.Source)
	if parseErr != nil {
		return fmt.Errorf("invalid source: %w", parseErr)
	}
	if entry.Branch != "" {
		source.Branch = entry.Branch
	}
	repoSubdir := strings.TrimSuffix(source.Subdir, entry.Subdir)
	repoSubdir = strings.TrimRight(repoSubdir, "/")
	source.Subdir = repoSubdir

	// Discover agents — use subdir-scoped discovery for monorepo installs.
	var discovery *install.DiscoveryResult
	var discErr error
	if source.HasSubdir() {
		discovery, discErr = install.DiscoverFromGitSubdir(source)
	} else {
		discovery, discErr = install.DiscoverFromGit(source)
	}
	if discErr != nil {
		return fmt.Errorf("discovery failed: %w", discErr)
	}
	defer install.CleanupDiscovery(discovery)

	// Find the specific agent by name
	agentName := filepath.Base(r.Name)
	var targetAgent *install.AgentInfo
	for i, a := range discovery.Agents {
		if a.Name == agentName {
			targetAgent = &discovery.Agents[i]
			break
		}
	}
	if targetAgent == nil {
		return fmt.Errorf("agent %q not found in repository", agentName)
	}

	// For grouped agents (r.Name contains "/", e.g. "tools/reviewer"),
	// reconstruct the correct destination subdirectory so the file lands
	// at agents/tools/reviewer.md rather than agents/reviewer.md.
	destDir := agentsDir
	if dir := filepath.Dir(r.Name); dir != "." {
		destDir = filepath.Join(agentsDir, dir)
	}

	installOpts := install.InstallOptions{
		Kind:      "agent",
		Force:     true,
		SourceDir: agentsDir,
	}
	_, installErr := install.InstallAgentFromDiscovery(discovery, *targetAgent, destDir, installOpts)
	return installErr
}

// updateAgentArgs holds parsed arguments for agent update.
type updateAgentArgs struct {
	names      []string
	groups     []string
	all        bool
	dryRun     bool
	jsonOutput bool
}

func parseUpdateAgentArgs(args []string) (*updateAgentArgs, bool, error) {
	opts := &updateAgentArgs{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all":
			opts.all = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--json":
			opts.jsonOutput = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.names = append(opts.names, arg)
		}
	}

	if !opts.all && len(opts.names) == 0 && len(opts.groups) == 0 {
		return nil, false, fmt.Errorf("specify agent name(s), --group, or --all")
	}
	if opts.all && (len(opts.names) > 0 || len(opts.groups) > 0) {
		return nil, false, fmt.Errorf("--all cannot be used with agent names or --group")
	}

	return opts, false, nil
}

func collectTrackedAgentResults(results []check.AgentCheckResult) []check.AgentCheckResult {
	tracked := make([]check.AgentCheckResult, 0, len(results))
	for _, r := range results {
		if r.Source != "" {
			tracked = append(tracked, r)
		}
	}
	return tracked
}

func filterAgentCheckResults(results []check.AgentCheckResult, names []string) []check.AgentCheckResult {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
		// Also index without .md suffix so "demo/tutor.md" matches "demo/tutor"
		nameSet[strings.TrimSuffix(n, ".md")] = true
	}
	var filtered []check.AgentCheckResult
	for _, r := range results {
		// Match full path (e.g. "demo/code-reviewer") or basename (e.g. "code-reviewer")
		if nameSet[r.Name] || nameSet[filepath.Base(r.Name)] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// validateAgentGroups checks that each group name corresponds to a subdirectory
// under agentsDir. Returns normalized group names (trailing "/" stripped).
func validateAgentGroups(groups []string, agentsDir string) ([]string, error) {
	normalized := make([]string, len(groups))
	for i, group := range groups {
		group = strings.TrimSuffix(group, "/")
		info, err := os.Stat(filepath.Join(agentsDir, group))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("agent group %q not found in %s", group, agentsDir)
		}
		normalized[i] = group
	}
	return normalized, nil
}

func matchesAnyGroup(name string, groups []string) bool {
	for _, group := range groups {
		if strings.HasPrefix(name, group+"/") {
			return true
		}
	}
	return false
}

// filterAgentResultsByGroups filters agent check results to those in the given groups.
func filterAgentResultsByGroups(results []check.AgentCheckResult, groups []string, agentsDir string) ([]check.AgentCheckResult, error) {
	groups, err := validateAgentGroups(groups, agentsDir)
	if err != nil {
		return nil, err
	}
	var filtered []check.AgentCheckResult
	for _, r := range results {
		if matchesAnyGroup(r.Name, groups) {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// filterDiscoveredAgentsByGroups filters discovered agents to those in the given groups.
func filterDiscoveredAgentsByGroups(discovered []resource.DiscoveredResource, groups []string, agentsDir string) ([]resource.DiscoveredResource, error) {
	groups, err := validateAgentGroups(groups, agentsDir)
	if err != nil {
		return nil, err
	}
	var filtered []resource.DiscoveredResource
	for _, d := range discovered {
		if matchesAnyGroup(strings.TrimSuffix(d.RelPath, ".md"), groups) {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

func logUpdateAgentOp(cfgPath string, total, updated, failed int, dryRun bool, start time.Time) {
	status := "ok"
	if failed > 0 && updated > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "error"
	}
	e := oplog.NewEntry("update", status, time.Since(start))
	e.Args = map[string]any{
		"resource_kind":  "agent",
		"agents_total":   total,
		"agents_updated": updated,
		"agents_failed":  failed,
		"dry_run":        dryRun,
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func updateAgentsOutputJSON(updatable []check.AgentCheckResult, dryRun bool, start time.Time, err error) error {
	type agentUpdateJSON struct {
		Name   string `json:"name"`
		Source string `json:"source,omitempty"`
		Status string `json:"status"`
	}
	var items []agentUpdateJSON
	for _, r := range updatable {
		items = append(items, agentUpdateJSON{
			Name:   r.Name,
			Source: r.Source,
			Status: r.Status,
		})
	}
	output := struct {
		Agents   []agentUpdateJSON `json:"agents"`
		DryRun   bool              `json:"dry_run"`
		Duration string            `json:"duration"`
	}{
		Agents:   items,
		DryRun:   dryRun,
		Duration: formatDuration(start),
	}
	return writeJSONResult(&output, err)
}

// cmdUpdateAgentsProject handles "skillshare update -p agents [name|--all]".
func cmdUpdateAgentsProject(args []string, projectRoot string, start time.Time) error {
	agentsDir := filepath.Join(projectRoot, ".skillshare", "agents")
	if _, err := os.Stat(agentsDir); err != nil {
		if os.IsNotExist(err) {
			ui.Info("No project agents directory (%s)", agentsDir)
			return nil
		}
		return fmt.Errorf("cannot access project agents: %w", err)
	}

	opts, showHelp, parseErr := parseUpdateAgentArgs(args)
	if showHelp {
		printUpdateHelp()
		return nil
	}
	if parseErr != nil {
		return parseErr
	}

	results := check.CheckAgents(agentsDir)
	if len(results) == 0 {
		ui.Info("No project agents found")
		return nil
	}

	if len(opts.names) > 0 {
		results = filterAgentCheckResults(results, opts.names)
		if len(results) == 0 {
			return fmt.Errorf("no matching agents found: %s", strings.Join(opts.names, ", "))
		}
	}

	if len(opts.groups) > 0 {
		var err error
		results, err = filterAgentResultsByGroups(results, opts.groups, agentsDir)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			return fmt.Errorf("no agents found in group(s): %s", strings.Join(opts.groups, ", "))
		}
	}

	tracked := collectTrackedAgentResults(results)

	if len(tracked) == 0 {
		ui.Info("No tracked project agents to update (all are local)")
		return nil
	}

	sp := ui.StartSpinner(fmt.Sprintf("Checking %d agent(s)...", len(tracked)))
	check.EnrichAgentResultsWithRemote(tracked, func() { sp.Success("Check complete") })

	var updatable []check.AgentCheckResult
	for _, r := range tracked {
		if r.Status == "update_available" {
			updatable = append(updatable, r)
		}
	}

	if len(updatable) == 0 {
		ui.Success("All project agents are up to date")
		return nil
	}

	ui.Header("Updating project agents")
	if opts.dryRun {
		ui.Warning("Dry run mode")
		for _, r := range updatable {
			ui.Info("  %s: update available from %s", r.Name, r.Source)
		}
		return nil
	}

	updated, failed := batchUpdateAgents(agentsDir, updatable, true)

	logUpdateAgentOp(config.ProjectConfigPath(projectRoot), len(updatable), updated, failed, opts.dryRun, start)

	if failed > 0 {
		return fmt.Errorf("%d agent(s) failed to update", failed)
	}
	return nil
}
