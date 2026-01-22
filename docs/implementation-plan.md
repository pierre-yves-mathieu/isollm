# isollm Implementation Plan

## Dependencies
- airyra SDK (task queue)
- lxc-dev-manager SDK (containers)
- typer (CLI)
- pyyaml (config)
- rich (output)

## Commands to Implement

### Core
- [ ] `isollm init` - Create isollm.yaml, .isollm/
- [ ] `isollm up` - Start session (containers + zellij + airyra)
- [ ] `isollm down` - Stop session, salvage prompts
- [ ] `isollm status` - Dashboard view

### Task
- [ ] `isollm task add <title>` - Add to queue
- [ ] `isollm task list` - Show tasks by status
- [ ] `isollm task clear` - Remove completed

### Worker
- [ ] `isollm worker add [name]` - Add container
- [ ] `isollm worker list` - Show workers
- [ ] `isollm worker shell <name>` - lxc exec
- [ ] `isollm worker logs <name>` - View logs
- [ ] `isollm worker reset <name>` - Reset to snapshot
- [ ] `isollm worker remove <name>` - Delete container

### Sync
- [ ] `isollm sync status` - Branch state
- [ ] `isollm sync pull` - Fetch task branches to host
- [ ] `isollm sync push` - Push host to bare repo

### Config
- [ ] `isollm config show` - Print config
- [ ] `isollm config edit` - Open in $EDITOR

## Core Modules

### bare_repo.py
- Create bare clone from working repo
- Set `gc.auto 0`
- Fetch/push operations

### zellij.py
- Generate layout KDL (dashboard + worker panes)
- Start/attach session

### project.py
- Load/validate isollm.yaml
- Defaults handling

### state.py
- .isollm/ directory management
- Worker state tracking
