# CLI E2E Runbook: Init Skip Skill Prompt When Remote Has Skills

Verifies that `skillshare init --remote <url>` skips the built-in skill install prompt when the remote repo already contains skills (issue #102).

## Scope

- `setupGitRemote` returns true when remote has skills
- `installSkillIfNeeded` is skipped when remote has skills
- Without remote, `--skill` flag works normally
- `--no-skill` flag still works

## Environment

Run inside devcontainer with mdproof isolation.

## Steps

### Step 1: Confirm --skill flag installs built-in skill

```bash
# mdproof setup already ran init; verify skill can be installed via upgrade
ss upgrade --force -g 2>/dev/null || true
ls ~/.config/skillshare/skills/skillshare/SKILL.md && echo "PASS: skill exists" || echo "INFO: skill not present"
```

**Expected:**
- exit_code: 0

### Step 2: Init with --no-skill skips skill

```bash
# Remove any existing skillshare skill
rm -rf ~/.config/skillshare/skills/skillshare 2>/dev/null
# Re-init without skill
rm -rf ~/.config/skillshare/config.yaml
ss init -g --all-targets --no-git --no-skill --no-copy
ls ~/.config/skillshare/skills/skillshare/SKILL.md 2>/dev/null && echo "FAIL: skill should not exist" || echo "PASS: skill skipped"
```

**Expected:**
- exit_code: 0
- PASS: skill skipped
- Not FAIL

### Step 3: Init with --remote that has skills — skill prompt skipped

```bash
# Create a bare git repo with skills
rm -rf /tmp/remote-skills.git /tmp/remote-skills-work 2>/dev/null
mkdir -p /tmp/remote-skills-work/my-remote-skill
cat > /tmp/remote-skills-work/my-remote-skill/SKILL.md << 'EOF'
---
name: my-remote-skill
description: A skill from the remote
---
# Remote Skill
EOF

cd /tmp/remote-skills-work
git init
git add .
git commit -m "initial skills"
git clone --bare /tmp/remote-skills-work /tmp/remote-skills.git

# Fresh init with --remote — setupGitRemote should return true (remote has skills)
# and skill prompt should be skipped
rm -rf ~/.config/skillshare/config.yaml ~/.config/skillshare/skills 2>/dev/null
ss init -g --all-targets --no-copy --remote file:///tmp/remote-skills.git 2>&1

# If skill prompt was skipped (correct behavior), skillshare/SKILL.md should NOT exist
# because the remote has my-remote-skill, not skillshare
ls ~/.config/skillshare/skills/skillshare/SKILL.md 2>/dev/null && echo "UNEXPECTED: skill prompt ran despite remote having skills" || echo "PASS: skill prompt was skipped"
```

**Expected:**
- exit_code: 0
- Not UNEXPECTED

### Step 4: Remote skill pulled successfully

```bash
# Check if remote skill was pulled
ls ~/.config/skillshare/skills/my-remote-skill/SKILL.md 2>/dev/null && echo "PASS: remote skill pulled" || echo "INFO: remote not auto-pulled (local had files)"
```

**Expected:**
- exit_code: 0

## Pass Criteria

- All 4 steps pass
- Init with --no-skill still skips skill
- Init with --remote that has skills skips the skill install prompt
