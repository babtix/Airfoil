---
id: "b2ee57873c3526ce"
title: "Minima-KV: Retention-Preserving KV Cache Compression with Mixed-Format Paged Attention"
summary: "Four new papers introduce methods to compress or reclaim key-value cache memory, directly addressing the storage and bandwidth limits that constrain long-context LLM serving. By applying mixed-format paging, dynamic reclamation, or low-rank page decomposition, engineering teams can reduce inference memory requirements and extend supported context lengths without modifying base models."
score: 47
tier: "notable"
tags: ["research", "infra", "models"]
builder_relevant: true
date: 2026-08-26T04:00:00Z
cluster_size: 4
sources:
  - name: "arXiv cs.AI"
    url: "https://arxiv.org/abs/2608.23834"
    tier: 2
    type: "lab"
  - name: "arXiv cs.AI"
    url: "https://arxiv.org/abs/2608.23658"
    tier: 2
    type: "lab"
  - name: "arXiv cs.AI"
    url: "https://arxiv.org/abs/2607.24555"
    tier: 2
    type: "lab"
  - name: "arXiv cs.LG"
    url: "https://arxiv.org/abs/2608.23843"
    tier: 2
    type: "lab"
takeaways:
  - "Mixed-format paging preserves recent tokens in FP8"
  - "Runtime reclamation recovers idle prefill memory"
  - "Low-rank sketches and decomposition shrink per-page storage"
---
