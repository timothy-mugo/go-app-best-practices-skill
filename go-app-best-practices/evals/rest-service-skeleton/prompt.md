---
tags: [skill-creator]
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

i need to start a new go service for managing customer invoices - REST API, postgres later but not yet. can you scaffold the project in the current dir (go mod name github.com/acme/invoices) with a main, a health endpoint, and a basic GET/POST /invoices handler with in-memory storage. keep it production-minded, we'll deploy it on k8s
