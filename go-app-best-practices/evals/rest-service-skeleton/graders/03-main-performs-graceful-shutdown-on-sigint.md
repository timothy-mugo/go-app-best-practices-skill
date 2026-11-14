---
type: llm
focus: trace
weight: 1
---

main performs graceful shutdown on SIGINT/SIGTERM via srv.Shutdown with a deadline
