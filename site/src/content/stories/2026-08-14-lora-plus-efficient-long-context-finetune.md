---
id: "1ae4f9805f7b4e90"
title: "LoRA+: 90% fewer params for 1M+ context fine-tuning"
summary: "Stanford NLP introduces LoRA+ — a parameter-efficient fine-tuning method for models with >1M context. Reduces trainable parameters by 90% versus full fine-tuning while matching performance. Includes custom CUDA kernels for 2x training speedup. Code and paper released."
score: 71
tier: "major"
tags: ["fine-tuning", "lora", "context-window", "research"]
builder_relevant: true
date: 2026-08-13T00:00:00Z
cluster_size: 1
sources:
  - name: "arXiv cs.CL"
    url: "https://arxiv.org/abs/2608.08765"
    tier: 2
  - name: "Stanford NLP GitHub"
    url: "https://github.com/stanford-nlp/lora-plus"
    tier: 3
takeaways:
  - "90% fewer trainable params vs full fine-tuning"
  - "Matches full fine-tuning performance at 1M+ context"
  - "Custom CUDA kernels for 2x training speedup"
  - "Code and paper open sourced"
---