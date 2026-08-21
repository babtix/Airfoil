---
id: "b936a5c8ba7ef604"
title: "LoRA-CRAFT: Cross-layer rank adaptation via frozen Tucker"
summary: "LoRA-CRAFT proposes a parameter‑efficient fine‑tuning technique that applies frozen Tucker tensor decomposition to the attention weights of a pre‑trained model, adapting ranks across layers to reduce trainable parameters while preserving performance."
score: 22
tier: "minor"
tags: ["models", "research", "tools"]
builder_relevant: true
date: 2026-08-19T04:00:00Z
cluster_size: 1
sources:
  - name: "arXiv cs.LG"
    url: "https://arxiv.org/abs/2602.17510"
    tier: 2
    type: "lab"
takeaways:
  - "LoRA-CRAFT uses frozen Tucker decomposition on attention weights for cross-layer rank adaptation."
  - "The method achieves extreme parameter efficiency in fine‑tuning large models."
  - "It maintains model performance while cutting trainable parameters."
---
