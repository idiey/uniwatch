# UniWatch — Local Setup & Claude Code Quickstart

## Step 1 — Install Claude Code

```bash
npm install -g @anthropic-ai/claude-code
```

## Step 2 — Set Up Both Repos Locally

```bash
# Hub repo
unzip uniwatch-hub.zip
cd uniwatch-hub
git init
git add .
git commit -m "chore: initial hub scaffold"
git branch -M main
git remote add origin git@github.com:idiey/uniwatch-hub.git
git push -u origin main
git checkout -b develop
git push -u origin develop
cd ..

# Agent repo
unzip uniwatch-agent.zip
cd uniwatch-agent
git init
git add .
git commit -m "chore: initial agent scaffold"
git branch -M main
git remote add origin git@github.com:idiey/uniwatch-agent.git
git push -u origin main
git checkout -b develop
git push -u origin develop
```

## Step 3 — Launch Claude Code

```bash
# For hub work
cd uniwatch-hub
claude

# For agent work
cd uniwatch-agent
claude
```

## Step 4 — Copy CLAUDE.md Into Each Repo

Before starting any sprint:

```bash
# Hub
cp /path/to/this-repo/hub/CLAUDE.md ~/uniwatch-hub/CLAUDE.md

# Agent
cp /path/to/this-repo/agent/CLAUDE.md ~/uniwatch-agent/CLAUDE.md
```

Then paste the relevant sprint prompt from `docs/sprints/` into the Claude Code session.

## Quick Reference

| Command | What it does |
|---|---|
| `make test-go` | Run Go tests with coverage |
| `make test-react` | Run React tests |
| `make test` | Both |
| `make build` | Build hub binaries |
| `make build-all` | Build agent binaries (all platforms) |
| `make lint` | Run all linters |
| `make fmt` | Format all code |
| `make dev` | Start all hub services locally |
| `make migrate ENV=dev` | Run DB migrations |

## Important Notes for Claude Code Sessions

1. **Always read CLAUDE.md first** — it is the single source of truth
2. **Check existing code before writing** — many interfaces and stubs are already in place
3. **Run tests after every task** — do not proceed if tests fail
4. **Never skip error handling** — every error must be handled
5. **Check the ENG-SPEC documents in docs/** for exact acceptance criteria
6. **Commit after each task** — do not accumulate large uncommitted changes
