# Bare Repo Module Plan

## Overview

Bare repo acts as hub between host working directory and worker containers.

```
Host ~/project ←→ ~/.isollm/project.git ←→ Containers /repo.git (mounted)
```

## Functions

### create(project_path, bare_path)
- `git clone --bare <project_path> <bare_path>`
- `git config gc.auto 0` (prevent corruption from concurrent pushes)
- Return bare repo path

### exists(bare_path) -> bool
- Check if bare repo already exists

### is_host_ahead(project_path, bare_path) -> int
- Compare host HEAD vs bare repo
- Return number of commits host is ahead
- Used for stale repo warning on `isollm up`

### push_to_bare(project_path, bare_path, branch="main")
- Push host branch to bare repo
- `git push <bare_path> <branch>`

### pull_from_bare(project_path, bare_path)
- Fetch all `isollm/*` branches from bare repo
- `git fetch <bare_path> 'refs/heads/isollm/*:refs/remotes/isollm/*'`

### list_task_branches(bare_path) -> list
- List all branches matching `isollm/*` pattern
- Return branch names with commit info

### delete_branch(bare_path, branch_name)
- Delete a task branch from bare repo
- Used during worker reset

### get_mount_path(project_name) -> str
- Return standard bare repo location: `~/.isollm/<project>.git`

## Safety Checks

- Never auto-gc (set on creation)
- Warn if host has unpushed commits before `up`
- Check for unpushed worker commits before `down --destroy`

## Usage Flow

```
isollm up:
  1. create() if not exists()
  2. is_host_ahead() → warn if > 0
  3. Mount bare_path into containers

isollm sync push:
  1. push_to_bare()

isollm sync pull:
  1. pull_from_bare()

isollm sync status:
  1. is_host_ahead()
  2. list_task_branches()

isollm worker reset:
  1. delete_branch() for worker's task branch
```
