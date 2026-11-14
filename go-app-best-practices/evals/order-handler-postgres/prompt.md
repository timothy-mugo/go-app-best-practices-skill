---
tags: [skill-creator]
max_turns: 30
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill, Write, Edit, Bash]
---

write me the order creation piece of our go backend: POST /orders takes a customer_id and a list of {sku, qty}, checks stock in postgres, decrements it and inserts the order + order lines atomically, and returns 201 with the new order. use database/sql with pgx stdlib driver. put the code under ./orders (handler, service, repository). no need to run it against a real db
