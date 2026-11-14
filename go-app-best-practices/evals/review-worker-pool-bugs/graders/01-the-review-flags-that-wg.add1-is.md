---
type: llm
focus: trace
weight: 1
---

The review flags that wg.Add(1) is called inside the goroutine so wg.Wait() can return before goroutines register
