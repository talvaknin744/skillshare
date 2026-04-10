---
sidebar_position: 2
---

# backup

Create, list, and manage backups of target directories.

```bash
skillshare backup              # Backup all skill targets
skillshare backup claude       # Backup specific target
skillshare backup agents       # Backup all agent targets
skillshare backup --all        # Backup skills + agents
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
```

## When to Use

- Create a manual backup before risky changes
- List existing backups to check recovery options
- Clean up old backups to free disk space

## Automatic Backups

Backups are created **automatically** before:
- `skillshare sync` (skill targets and agent targets)
- `skillshare sync agents` (agent targets only)
- `skillshare target remove`

Location: `~/.local/share/skillshare/backups/<timestamp>/`

## Commands

### Create Backup

```bash
skillshare backup              # All targets
skillshare backup claude       # Specific target
skillshare backup --dry-run    # Preview
```

### List Backups

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.config/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.config/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.config/.../2026-01-18_09-00-00
```

### Cleanup Old Backups

```bash
skillshare backup --cleanup           # Remove old backups
skillshare backup --cleanup --dry-run # Preview cleanup
```

Default cleanup policy:
- Keep last 10 backups
- Remove backups older than 30 days
- Cap total size at 500 MB

## Options

| Flag | Description |
|------|-------------|
| `--all` | Backup both skills and agents |
| `--project, -p` | Use project mode (`.skillshare/backups/`); **agents only** |
| `--global, -g` | Use global mode (default for skills) |
| `--list, -l` | List all backups |
| `--cleanup, -c` | Remove old backups |
| `--target, -t <name>` | Target specific backup (alternative to positional arg) |
| `--dry-run, -n` | Preview without making changes |

`backup` also accepts a positional kind argument: `skillshare backup agents` scopes the backup to agent targets only.

## Backup Structure

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

## What Gets Backed Up

- Regular directories in targets (actual skill files)
- Per-skill symlinks in merge-mode targets are **resolved and followed** — the real content is backed up

This means:
- In merge mode: All skills are backed up (symlinks resolved, local copies included)
- In copy mode: All managed skill directories are backed up
- In symlink mode: Nothing is backed up (entire directory is a single symlink)

## Agent Backup

Agents have their own backup flow that runs alongside skill backups, with two distinctions worth knowing:

**Entry naming.** Agent backups are stored under `<target>-agents/` inside each timestamp directory, parallel to the skill backup. For example, after `skillshare backup --all` the layout looks like:

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # Skills backup for claude
├── claude-agents/   # Agents backup for claude
└── cursor/
```

**Project mode is the inverse of skills.** In project mode (`-p`), `backup` refuses to back up skill targets but **does** back up agent targets. The error you'll see if you forget the `agents` filter:

```
backup is not supported in project mode (except for agents)
```

So in project mode you must say either `skillshare backup -p agents` or `skillshare backup -p --all`.

```bash
skillshare backup agents                  # All agent targets (global)
skillshare backup agents claude           # Only claude's agents
skillshare backup agents -p               # Project agent targets
skillshare backup --all                   # Skills + agents in one shot
```

See [Agents](/docs/understand/agents) for the agent resource model and [restore](/docs/reference/commands/restore) for recovery.

## See Also

- [restore](/docs/reference/commands/restore) — Restore from backup
- [sync](/docs/reference/commands/sync) — Auto-creates backups
- [target remove](/docs/reference/commands/target) — Auto-creates backups
