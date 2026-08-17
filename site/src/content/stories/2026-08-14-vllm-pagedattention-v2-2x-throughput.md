---
id: "d6a0f54c1b370a5c"
title: "vLLM 0.7 brings PagedAttention v2: 2x throughput for long context"
summary: "vLLM 0.7 releases PagedAttention v2 with native support for 1M+ context windows. Benchmarks show 2x throughput improvement over v1 for 128K+ context. New KV cache compression reduces memory by 40%. The inference engine now powers production Llama 4 deployments at 10M context."
score: 82
tier: "major"
tags: ["infra", "inference", "vllm", "context-window"]
builder_relevant: true
date: 2026-08-14T09:30:00Z
cluster_size: 1
sources:
  - name: "GitHub Trending"
    url: "https://github.com/vllm-project/vllm"
    tier: 3
    metrics: { github_stars: 89300 }
takeaways:
  - "PagedAttention v2: 2x throughput for 128K+ context"
  - "KV cache compression reduces memory by 40%"
  - "Native support for 1M+ context windows"
  - "Powers Llama 4 10M context deployments"
---