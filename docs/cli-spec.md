# CLI Design: isollm (Isolated LLM Orchestrator)

## Philosophy

- **Intent-based commands** - Commands describe what you want, not how it works
- **Sensible defaults** - Works out of the box, configure when needed
- **Progressive disclosure** - Simple commands for common tasks, flags for power users
- **Unified experience** - One tool, not three tools duct-taped together
- **Host as hub** - Your local repo is the source of truth; workers sync to it

---

## Command Structure Overview

```
isollm
├── init                    # Initialize project
├── up                      # Start session (containers + zellij + airyra)
├── down                    # Stop session gracefully
├── status                  # Dashboard view of everything
│
├── task                    # Task management
│   ├── add <title>         # Add task to queue
│   ├── list                # Show all tasks
│   └── clear               # Clear completed tasks
│
├── worker                  # Worker/container management
│   ├── add [name]          # Add another worker
│   ├── list                # List workers and their status
│   ├── shell <name>        # Shell into worker (alias: ssh)
│   ├── logs <name>         # View worker logs
│   ├── reset <name>        # Reset worker to clean state
│   └── remove <name>       # Remove a worker
│
├── sync                    # Git sync with bare repo
│   ├── status              # Branch status across workers
│   ├── pull                # Fetch task branches to host
│   └── push                # Push host changes to bare repo
│
└── config                  # Configuration
    ├── show                # Show current config
    └── edit                # Open config in editor
```

---

## Git Model: Bare Repo Hub + Branch Per Task

**How it works (no config needed):**

```
┌────────────────────────────────────────────────────────────────┐
│                            HOST                                 │
│                                                                 │
│   ~/my-project/                    ~/.isollm/my-project.git     │
│   (your working dir)               (bare repo - auto created)   │
│   ├── .git/                        ├── objects/                 │
│   ├── src/                         ├── refs/heads/              │
│   └── isollm.yaml                  │   ├── main                 │
│                                    │   ├── isollm/ar-a1b2       │
│         ↑                          │   ├── isollm/ar-c3d4       │
│         │ isollm sync pull         │   └── isollm/ar-e5f6       │
│         │ (fetch branches)         │                            │
│         │                          │         ↑                  │
│         │ isollm sync push         │         │                  │
│         │ (send host changes) ─────┼─────────┘                  │
│         └──────────────────────────┤                            │
│                                    │                            │
│                              LXC disk mount                     │
│                    ┌───────────────┼───────────────┐            │
│                    ↓               ↓               ↓            │
│               worker-1        worker-2        worker-3          │
│               /repo.git       /repo.git       /repo.git         │
│                   │               │               │             │
│            task ar-a1b2     task ar-c3d4    task ar-e5f6        │
│            (own branch)     (own branch)    (own branch)        │
└────────────────────────────────────────────────────────────────┘
```

**Branch per task (not per worker):**
- Worker claims task `ar-a1b2` → creates branch `isollm/ar-a1b2`
- Worker completes task → pushes branch → claims next task → new branch
- Each task = isolated branch = easy to merge, revert, or cherry-pick

**The flow:**
1. `isollm up` creates bare repo clone, warns if host has unpushed changes
2. Bare repo is mounted into each container at `/repo.git`
3. Worker claims task → creates branch `isollm/<task-id>`
4. Worker pushes to `/repo.git` (instant filesystem write)
5. `isollm sync pull` fetches task branches into your working repo
6. You merge with standard `git merge` commands

**Bidirectional sync:**
```bash
isollm sync push   # Host changes → bare repo (before workers need them)
isollm sync pull   # Task branches → host (to review/merge)
```

**Benefits:**
- No GitHub/GitLab required
- No SSH keys or network setup
- Instant push/pull (filesystem, not network)
- Works completely offline
- Clean history: one branch per task

**Optional upstream:** Also push to GitHub:
```yaml
git:
  upstream: origin
```

---

## Core Commands

### `isollm init`

Initialize a new isollm project in current directory.

```bash
isollm init
```

**Interactive prompts:**
- Project name (default: directory name)
- Number of workers (default: 3)
- Base image (default: ubuntu:24.04)

**Creates:**
- `isollm.yaml` - Project configuration
- `.isollm/` - Local state directory

**Example config generated:**
```yaml
project: my-project
workers: 3
image: ubuntu:24.04

# Git configuration
git:
  base_branch: main    # Workers fork task branches from here
                       # Change to 'master' if that's your default branch
  # upstream: origin   # Uncomment to also push to GitHub/GitLab

claude:
  command: claude
  args: []

ports:
  - 3000
  - 8080
```

---

### `isollm up`

Start the orchestration session.

```bash
isollm up                      # Start with defaults from config
isollm up -n 5                 # Override: start 5 workers
isollm up --base develop       # Fork from 'develop' instead of configured base
```

**What happens:**
1. Checks if host repo has changes not in bare repo → warns to run `isollm sync push`
2. Starts airyra server (if not running)
3. Creates bare repo (if first run) with `gc.auto 0` to prevent corruption
4. Configures UID mapping for container↔host file permissions
5. Creates/starts N containers via lxc-dev-manager
6. Mounts bare repo into each container
7. Launches zellij with auto-generated layout
8. Each pane runs Claude with airyra integration

**Stale repo warning:**
```
$ isollm up
Warning: Host has 2 commits not in bare repo.
Workers will not see these changes.
Run 'isollm sync push' first, or --force to continue anyway.
```

**Zellij layout generated:**
```
┌─────────────────────────────────────────────────┐
│                 Task Dashboard                   │
│  READY: 5  IN_PROGRESS: 2  DONE: 3              │
│  → worker-1: Implementing auth middleware       │
│  → worker-2: Writing JWT token service          │
│  → worker-3: (claiming next task...)            │
├────────────────┬────────────────┬───────────────┤
│   worker-1     │   worker-2     │   worker-3    │
│   [claude]     │   [claude]     │   [claude]    │
│                │                │               │
└────────────────┴────────────────┴───────────────┘
```

**The CLI is just a CLI** - humans and Claude both use it:
```bash
# Human adds a task
isollm task add "Implement auth"

# Claude (in any pane) can do the same
# Workers can add sub-tasks as they discover work
```

No enforced planner role. Any Claude instance can add tasks, claim tasks, etc.

---

### `isollm down`

Gracefully stop the session.

```bash
isollm down            # Stop all, keep containers
isollm down --destroy  # Stop and remove containers (with confirmation)
isollm down --save     # Snapshot all workers before stopping
```

**What happens:**
1. Workers push any uncommitted work
2. Releases any claimed tasks back to queue
3. Stops zellij session
4. Optionally snapshots/destroys containers

**Destroy confirmation:**
```
$ isollm down --destroy
This will permanently delete 3 containers:
  worker-1  (task branch isollm/ar-a1b2 has 3 unpushed commits)
  worker-2  (no unpushed commits)
  worker-3  (task branch isollm/ar-e5f6 has 1 unpushed commits)

Type 'destroy' to confirm:
```

Use `--yes` to skip confirmation (for scripts).

---

### `isollm status`

Show current state dashboard.

```bash
isollm status          # Full dashboard
isollm status --brief  # One-line summary
isollm status --json   # Machine-readable
```

**Output:**
```
isollm: my-project
═══════════════════════════════════════════════════

Workers (3 running):
  worker-1  ● running   192.168.1.101
            └─ task: ar-a1b2 "Add user auth" (12m) → isollm/ar-a1b2
  worker-2  ● running   192.168.1.102
            └─ task: ar-c3d4 "Write API tests" (5m) → isollm/ar-c3d4
  worker-3  ● running   192.168.1.103
            └─ (idle, waiting for task)

Tasks:
  Ready:        5
  In Progress:  2
  Blocked:      1
  Completed:    12

Sync:
  Host: main @ abc1234 (in sync with bare repo)
  Task branches: 8 (6 merged, 2 in progress)

Airyra: ● running (localhost:7432)
Zellij: ● attached (session: my-project)
```

---

## Task Commands

Tasks flow through airyra. The CLI works for humans and Claude alike:

```bash
isollm task add "..."    # Human or Claude can add tasks
isollm task list         # See the queue
# Workers claim → execute → done
```

### `isollm task list`

See what Claude planned and what's happening.

```bash
isollm task list                # All tasks
isollm task list --ready        # Claimable
isollm task list --in-progress  # Being worked
isollm task list --done         # Completed
```

**Output:**
```
Tasks: my-project
─────────────────────────────────────────────────

Ready (5):
  ar-a1b2  [high]    Implement user authentication
  ar-c3d4  [normal]  Add input validation
  ar-e5f6  [normal]  Create dashboard component

In Progress (2):
  ar-k1l2  [normal]  Add user authentication    → worker-1 (12m)
  ar-m3n4  [normal]  Write API tests            → worker-2 (5m)

Blocked (1):
  ar-o5p6  [normal]  Deploy to staging          ⊘ depends on: ar-k1l2

Done (12): use --done to show
```

---

### `isollm task add`

Add a task to the queue.

```bash
isollm task add "Implement login endpoint"
isollm task add "Fix urgent bug" -p critical
isollm task add "Write tests" --depends-on ar-abc1
```

**Flags:**
- `-p, --priority`: critical, high, normal (default), low
- `-d, --depends-on`: Task ID this depends on
- `-D, --description`: Longer description

---

### `isollm task clear`

Remove completed tasks from the queue.

```bash
isollm task clear          # Clear all done tasks
isollm task clear --all    # Clear everything (reset queue)
```

---

## Worker Commands

### `isollm worker add`

Add more workers to running session.

```bash
isollm worker add              # Add one with auto-generated name
isollm worker add frontend     # Add with specific name
isollm worker add -n 2         # Add two workers
```

---

### `isollm worker shell`

Open a shell into a specific worker (uses `lxc exec` under the hood).

```bash
isollm worker shell worker-1                # Interactive shell
isollm worker shell worker-1 -c "git log"   # Run command
```

**Alias:** `isollm worker ssh` works too.

---

### `isollm worker reset`

Reset a worker to clean state.

```bash
isollm worker reset worker-1              # Reset to initial snapshot
isollm worker reset --all                 # Reset all workers
```

**What happens:**
1. Checks for uncommitted/unpushed work
2. Offers to salvage: push current branch before reset
3. Releases any claimed task
4. Deletes task branch in bare repo (clean slate)
5. Restores container from snapshot (instant with ZFS)
6. Fresh clone from bare repo
7. Restarts Claude

**Salvage prompt:**
```
$ isollm worker reset worker-1
Worker has unpushed changes on branch isollm/ar-a1b2:
  3 commits, 5 files changed

Options:
  [s] Salvage - push branch before reset (can merge later)
  [d] Discard - delete branch and reset
  [c] Cancel

Choice:
```

---

### `isollm worker logs`

View what a worker has been doing.

```bash
isollm worker logs worker-1           # Recent activity
isollm worker logs worker-1 -f        # Stream live
```

---

## Sync Commands

Sync between host repo and bare repo. Workers push to bare repo; you fetch from it.

**Note:** Merging is done with standard `git merge` - no wrapper needed.

### `isollm sync status`

See sync state between host, bare repo, and workers.

```bash
isollm sync status
```

**Output:**
```
Sync: my-project
─────────────────────────────────────────────────

Host repo: /home/user/my-project
  HEAD: main @ abc1234 "Initial commit"
  Status: ✓ in sync with bare repo

Bare repo: ~/.isollm/my-project.git
  Task branches:
    isollm/ar-a1b2  +3 commits  "Add user authentication"
                    └─ Done ✓ (ready to merge)
    isollm/ar-c3d4  +1 commits  "WIP: API tests"
                    └─ In progress (worker-2)
    isollm/ar-e5f6  +2 commits  "Input validation"
                    └─ Done ✓ (ready to merge)
```

---

### `isollm sync pull`

Fetch task branches from bare repo into host repo.

```bash
isollm sync pull              # Fetch all task branches
```

After this, branches appear as `isollm/ar-xxxx` in your host repo. Merge with standard git:

```bash
isollm sync pull
git merge isollm/ar-a1b2      # Merge completed task
git branch -d isollm/ar-a1b2  # Clean up
```

---

### `isollm sync push`

Push host changes to bare repo (so workers see them).

```bash
isollm sync push              # Push current branch to bare repo
```

Use when you've made commits on host that workers need:

```bash
# Made changes on host
git commit -m "Fix config"
isollm sync push              # Workers can now pull this
```

---

## Workflow Examples

### Quick Start (Zero Config)
```bash
cd my-project            # Any git repo
isollm init              # Accept defaults
isollm up                # Start session with zellij

# Add initial tasks (you or Claude)
isollm task add "Implement JWT auth service"
isollm task add "Add login endpoint"
isollm task add "Add protected route middleware"
isollm task add "Write auth tests"

# Workers claim tasks, create branches, execute
# Watch progress in dashboard pane
```

### Adding Work Mid-Session
```bash
# You or any Claude instance can add more work anytime:
isollm task add "Fix login bug reported by QA" -p critical
isollm task add "Add rate limiting to auth"

# Workers can also add sub-tasks as they discover them
```

### Scaling Up
```bash
isollm worker add -n 2   # Add 2 more workers if queue backs up
```

### Watching Progress
```bash
isollm status            # See task progress and branches
isollm sync status       # See detailed branch state
```

### Merging Results
```bash
isollm sync pull                 # Fetch all task branches to host
isollm sync status               # See which are ready

# Use standard git to merge
git merge isollm/ar-a1b2         # Merge completed task
git merge isollm/ar-c3d4         # Merge another
git branch -d isollm/ar-a1b2     # Clean up merged branches
git push origin main             # Push to upstream
```

### Sending Host Changes to Workers
```bash
# You made a fix on host that workers need
git commit -m "Fix shared config"
isollm sync push                 # Push to bare repo
# Workers can now: git pull origin main
```

---

## Configuration Reference

### `isollm.yaml`

```yaml
project: my-project

# Worker configuration
workers: 3
image: ubuntu:24.04
setup_script: |
  # Runs once when container is created
  npm install

# Git configuration
git:
  base_branch: main              # Branch workers fork from (default: main)
                                 # Change to 'master' or other for different setups
  branch_prefix: isollm/         # Task branch prefix (default: isollm/)
  upstream: origin               # Also push to this remote (optional)

# Claude configuration
claude:
  command: claude                # Command to run Claude
  args: []                       # Additional arguments

# Airyra configuration
airyra:
  project: my-project            # Airyra project name (default: same as project)

# Port forwarding (host:container)
ports:
  - 3000
  - 8080:8000

# Zellij layout
zellij:
  layout: auto                   # auto, horizontal, vertical, grid
  dashboard: true                # Show status pane
```

---

## Implementation Notes

### Git Sync: Bare Repo Pattern

Workers need to push/pull from the host. We use a **bare repo** as an intermediary - no SSH keys, no network, just filesystem.

**What is a bare repo?**

A bare repo is a Git repository without a working directory - just the `.git` contents. It's what GitHub uses internally. Safe to receive pushes because there are no working files to conflict.

```
Normal repo:                    Bare repo:
my-project/                     my-project.git/
├── .git/                       ├── HEAD
│   ├── objects/                ├── objects/
│   └── refs/                   ├── refs/
├── src/          ← files       └── config
└── README.md     ← files       (no working files)
```

**Architecture:**

```
┌────────────────────────────────────────────────────────────────┐
│                            HOST                                 │
│                                                                 │
│   ~/my-project/                    ~/.isollm/my-project.git     │
│   ├── .git/                        (bare repo)                  │
│   ├── src/           ← you ──→     ├── objects/                 │
│   └── README.md        work        ├── refs/heads/              │
│                        here        │   ├── main                 │
│        ↑                           │   ├── isollm/ar-a1b2       │
│        │                           │   ├── isollm/ar-c3d4       │
│        │ isollm sync pull          │   └── isollm/ar-e5f6       │
│        │ (fetches branches)        │                            │
│        │                           │                            │
│        └───────────────────────────┤                            │
│                                    │                            │
│                              LXC disk mount                     │
│                    ┌───────────────┼───────────────┐            │
│                    ↓               ↓               ↓            │
│               worker-1        worker-2        worker-3          │
│               /repo.git       /repo.git       /repo.git         │
│               (mounted)       (mounted)       (mounted)         │
│                   ↓               ↓               ↓             │
│               ~/project       ~/project       ~/project         │
│            branch: ar-a1b2  branch: ar-c3d4  branch: ar-e5f6   │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

**The flow:**

```bash
# 1. isollm up: create bare repo from your working repo
git clone --bare ~/my-project ~/.isollm/my-project.git
git -C ~/.isollm/my-project.git config gc.auto 0  # Prevent corruption

# 2. Mount bare repo into each container with UID mapping
lxc config set worker-1 raw.idmap "uid 1000 1000\ngid 1000 1000"
lxc config device add worker-1 repo disk \
    source=/home/user/.isollm/my-project.git \
    path=/repo.git

# 3. Worker claims task, creates branch
# (inside container)
git clone /repo.git ~/project
git checkout -b isollm/ar-a1b2   # Branch named after task ID

# 4. Worker pushes (writes directly to mounted bare repo)
git add . && git commit -m "Implement feature"
git push origin isollm/ar-a1b2

# 5. Host fetches task branches
# (on host)
git fetch ~/.isollm/my-project.git 'refs/heads/isollm/*:refs/remotes/isollm/*'

# 6. Host merges with standard git
git merge isollm/ar-a1b2
```

---

### Container UID Mapping

LXC unprivileged containers map UIDs differently. Without config, files created by container user 1000 appear as UID 101000 on host (unreadable).

**Solution:** Raw idmap to match UIDs:
```bash
lxc config set worker-1 raw.idmap "uid 1000 1000\ngid 1000 1000"
```

This makes container UID 1000 = host UID 1000. Files in the mounted bare repo are readable/writable by both.

---

### Preventing Git Corruption

Multiple workers pushing to the same bare repo can trigger auto-gc, potentially corrupting packfiles.

**Solution:** Disable auto-gc in bare repo:
```bash
git -C ~/.isollm/my-project.git config gc.auto 0
```

Manual gc runs during `isollm down` when no workers are active.

---

### Branch Per Task

Each task gets its own branch (`isollm/<task-id>`), not per worker.

**Benefits:**
- Clean history: one branch = one task
- Easy revert: `git revert` the merge commit
- Easy cherry-pick: grab specific task's changes
- No interleaved commits from different tasks

**Flow:**
1. Worker claims task `ar-a1b2`
2. Worker creates branch `isollm/ar-a1b2` from `main`
3. Worker completes task, pushes branch
4. Worker claims next task `ar-x9y8`
5. Worker creates new branch `isollm/ar-x9y8` from `main`

---

### Stale Repo Warning

If host has commits not in bare repo, workers won't see them.

`isollm up` checks this and warns:
```
Warning: Host has 2 commits not in bare repo.
Workers will not see these changes.
Run 'isollm sync push' first, or --force to continue anyway.
```

---

## Open Questions

1. **Naming**: `isollm` or alternatives?
   - `swarm`, `hive`, `forge`, `loom`, `parallel`

2. **Claude integration**: How does Claude know to use airyra?
   - Custom CLAUDE.md in each container?
   - System prompt injection?

3. **Dashboard**: TUI in zellij pane vs separate web UI?

## Resolved Decisions

| Question | Decision |
|----------|----------|
| Host↔container git sync | Bare repo with LXC disk mount |
| UID mapping | `raw.idmap "uid 1000 1000"` |
| Concurrent push safety | `gc.auto 0` in bare repo |
| Branch strategy | Branch per task (not per worker) |
| Host→bare sync | Manual `isollm sync push` + warning on `up` |
| Merge command | None - use standard `git merge` |
| Destructive commands | Require confirmation, `--yes` to skip |
| Worker reset | Offer salvage option before deleting branch |
| Command namespace | `isollm sync` (not `isollm git`) |
