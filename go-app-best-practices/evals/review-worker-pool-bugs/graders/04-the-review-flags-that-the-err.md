---
type: llm
focus: trace
weight: 1
---

The review flags that the err declared inside the goroutine shadows the outer err, so CheckAll always returns a nil error
