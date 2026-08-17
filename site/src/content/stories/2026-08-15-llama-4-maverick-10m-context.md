---
id: "c5f9e43b0a26f9b4"
title: "Meta drops Llama 4 Maverick: 400B open weights, 10M context"
summary: "Meta AI releases Llama 4 Maverick, a 400B parameter open-weight model with an unprecedented 10 million token context window — the largest of any open model. Uses RingAttention++ architecture and novel positional encoding. Weights available on Hugging Face under Llama 4 Community License. Community validates 89% needle-in-haystack recall at 8M tokens on a single RTX 4090 via 4-bit quantization."
score: 94
tier: "major"
tags: ["models", "meta", "open-source", "context-window", "quantization"]
builder_relevant: true
date: 2026-08-15T16:45:00Z
cluster_size: 6
sources:
  - name: "Meta AI Blog"
    url: "https://ai.meta.com/blog/llama-4-maverick-release/"
    tier: 1
  - name: "Hugging Face Papers"
    url: "https://huggingface.co/papers/2608.12345"
    tier: 2
    metrics: { hf_upvotes: 2341 }
  - name: "VentureBeat AI"
    url: "https://venturebeat.com/ai/meta-llama-4-maverick-10m-context-open-weights/"
    tier: 4
  - name: "Hacker News"
    url: "https://news.ycombinator.com/item?id=44892134"
    tier: 5
    metrics: { hn_points: 1240, hn_comments: 380 }
  - name: "r/LocalLLaMA"
    url: "https://www.reddit.com/r/LocalLLaMA/comments/1abcdef/llama_4_maverick_400b_running_on_24gb_vram/"
    tier: 5
    metrics: { reddit_score: 2890 }
  - name: "arXiv"
    url: "https://arxiv.org/abs/2608.12345"
    tier: 2
takeaways:
  - "10M context window — largest of any open model"
  - "400B params, open weights under Llama 4 Community License"
  - "Runs on single 24GB RTX 4090 via 4-bit quantization"
  - "RingAttention++ architecture with novel positional encoding"
---