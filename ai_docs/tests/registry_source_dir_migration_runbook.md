# CLI E2E Runbook: Registry Source Dir Migration

Verifies that `registry.yaml` is stored in the source/skills directory (not config directory) and that one-time migration from old to new location works correctly.

## Scope

- Fresh init: registry.yaml created in source dir (`~/.config/skillshare/skills/`)
- Install writes registry to source dir, NOT config dir
- Migration: old registry in config dir auto-moves to source dir on any command
- Both-exist: new location wins, warns about old
- Project mode: unchanged (`.skillshare/registry.yaml`)

## Environment

Run inside devcontainer with mdproof isolation.

## Steps

### Step 1: Fresh init — registry not in config dir

```bash
rm -rf /tmp/test-skill /tmp/proj-skill /tmp/project-test 2>/dev/null || true
# After init, registry may not exist yet (created on first install)
ls ~/.config/skillshare/skills/registry.yaml 2>/dev/null || echo "registry not yet created (expected on fresh init)"
# Confirm NOT in config dir
ls ~/.config/skillshare/registry.yaml 2>/dev/null && echo "FAIL: registry in config dir" || echo "PASS: not in config dir"
```

**Expected:**
- exit_code: 0
- PASS: not in config dir
- Not FAIL

### Step 2: Install writes registry to source dir

```bash
mkdir -p /tmp/test-skill
cat > /tmp/test-skill/SKILL.md << 'EOF'
---
name: test-skill
---
# Test Skill
EOF

ss install /tmp/test-skill --force -g
cat ~/.config/skillshare/skills/registry.yaml
ls ~/.config/skillshare/registry.yaml 2>/dev/null && echo "FAIL: leaked to config dir" || echo "PASS: source dir only"
```

**Expected:**
- exit_code: 0
- name: test-skill
- PASS: source dir only
- Not FAIL

### Step 3: Migration — old config dir registry auto-moves to source dir

```bash
# Simulate old layout: place registry in config dir, remove from source dir
cp ~/.config/skillshare/skills/registry.yaml ~/.config/skillshare/registry.yaml
rm -f ~/.config/skillshare/skills/registry.yaml

# Run any command to trigger config.Load() migration
ss status -g

# Verify old location is gone, new exists
ls ~/.config/skillshare/registry.yaml 2>/dev/null && echo "FAIL: old file still exists" || echo "PASS: migrated"
cat ~/.config/skillshare/skills/registry.yaml
```

**Expected:**
- exit_code: 0
- PASS: migrated
- name: test-skill
- Not FAIL

### Step 4: Both exist — new location wins

```bash
# Place registry in BOTH locations with different content
cat > ~/.config/skillshare/registry.yaml << 'YAML'
skills:
  - name: old-location-skill
    source: github.com/old/repo
YAML

cat > ~/.config/skillshare/skills/registry.yaml << 'YAML'
skills:
  - name: new-location-skill
    source: github.com/new/repo
YAML

# Run command — new location wins
ss list -g --json 2>/dev/null | head -1 || true

# New location content should be preserved
grep "new-location-skill" ~/.config/skillshare/skills/registry.yaml && echo "PASS: new preserved" || echo "FAIL: new lost"
# Old location should still exist (not auto-deleted when both present)
ls ~/.config/skillshare/registry.yaml && echo "PASS: old kept" || echo "FAIL: old deleted"

# Cleanup old file for subsequent steps
rm -f ~/.config/skillshare/registry.yaml
```

**Expected:**
- exit_code: 0
- PASS: new preserved
- PASS: old kept
- Not FAIL

### Step 5: Uninstall updates registry in source dir

```bash
mkdir -p /tmp/test-skill
cat > /tmp/test-skill/SKILL.md << 'EOF'
---
name: remove-me
---
# Remove Me
EOF
ss install /tmp/test-skill --force -g
ss uninstall remove-me --force -g
grep "remove-me" ~/.config/skillshare/skills/registry.yaml && echo "FAIL: still in registry" || echo "PASS: removed from registry"
```

**Expected:**
- exit_code: 0
- PASS: removed from registry
- Not FAIL

### Step 6: Project mode — registry in .skillshare/

```bash
rm -rf /tmp/project-test /tmp/proj-skill 2>/dev/null
mkdir -p /tmp/project-test
cd /tmp/project-test
ss init -p --targets claude

mkdir -p /tmp/proj-skill
cat > /tmp/proj-skill/SKILL.md << 'EOF'
---
name: proj-skill
---
# Project Skill
EOF

SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 ss install /tmp/proj-skill -p
cat /tmp/project-test/.skillshare/registry.yaml
```

**Expected:**
- exit_code: 0
- name: proj-skill

## Pass Criteria

- All 6 steps pass
- registry.yaml is always in source dir for global mode
- Migration from old config dir location works
- Both-exist scenario preserves new, keeps old
- Project mode unaffected
