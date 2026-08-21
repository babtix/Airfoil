---
id: "b8f430b9c87d64ea"
title: "Two papers improve on-policy distillation for long-context reasoning"
summary: "Researchers propose two enhancements to on-policy distillation for language models: group-calibrated OPD, which adjusts teacher guidance using ensemble statistics to avoid locally plausible but evidence‑missing answers, and a filtering method that retains only student trajectories showing measurable reasoning progress, improving long‑context reasoning performance."
score: 45
tier: "notable"
tags: ["research", "models", "tools"]
builder_relevant: true
date: 2026-08-21T04:00:00Z
cluster_size: 2
sources:
  - name: "arXiv cs.AI"
    url: "https://arxiv.org/abs/2608.19181"
    tier: 2
    type: "lab"
  - name: "arXiv cs.LG"
    url: "https://arxiv.org/abs/2608.19408"
    tier: 2
    type: "lab"
takeaways:
  - "Group-calibrated OPD reduces evidence omission in long-context tasks."
  - "Filtering by reasoning progress selects useful student trajectories."
  - "Both methods improve student model reasoning without extra teacher data."
---
