# go-app-best-practices

A Claude Code skill with best practices for building, reviewing and hardening Go applications: project layout, idiomatic code, errors, context, goroutines, HTTP, databases, caching, security, observability, testing and CI.

`SKILL.md` holds the non-negotiable rules and routes each task to the matching reference file, so only the relevant material is loaded.

## Install

With the [skills CLI](https://github.com/vercel-labs/skills):

```bash
npx skills add timothy-mugo/go-app-best-practices-skill
```

It prompts for the agent (Claude Code, etc.) and for project or global scope. Useful options:

```bash
npx skills add timothy-mugo/go-app-best-practices-skill --list   # show skills in the repo without installing
npx skills update                                                # pull the latest version
npx skills remove go-app-best-practices
```

The repo must be public, or your machine must have GitHub access to it, for the command to work.

### Manual install

Copy the `go-app-best-practices/` folder into a Claude Code skills directory, either `~/.claude/skills/` (all projects) or `.claude/skills/` (one project):

```bash
git clone https://github.com/timothy-mugo/go-app-best-practices-skill.git
mkdir -p ~/.claude/skills
cp -R go-app-best-practices-skill/go-app-best-practices ~/.claude/skills/
```

Restart Claude Code. The skill triggers automatically on Go work (`.go` files, `go.mod`, goroutines, `net/http`, building or reviewing a Go service).

## Contents

```
go-app-best-practices/
├── SKILL.md        rules, workflow, routing table
├── references/     Go-only guides and the guidelines/ tree (error handling, concurrency, testing, ...)
├── assets/         Makefile and golangci.yml to copy into projects
└── evals/          test cases used to evaluate the skill
```

## Attribution

Adapted from [nutcas3/best-practices](https://github.com/nutcas3/best-practices). The files under `references/guidelines/` come from the sw-factory agentic-orchestrator Go guidelines. `references/uber-style-guide.md` is a condensed paraphrase of the [Uber Go Style Guide](https://github.com/uber-go/guide) (Apache 2.0, Uber Technologies).
