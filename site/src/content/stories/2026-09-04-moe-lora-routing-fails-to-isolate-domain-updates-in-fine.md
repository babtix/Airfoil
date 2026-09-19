---
id: "99a26eed359d6de9"
title: "MoE+LoRA Routing Fails to Isolate Domain Updates in Fine-Tuning"
summary: "Research shows token-level routing in Mixture-of-Experts combined with LoRA does not isolate domain-specific updates, causing intra-adapter subspace contention. Builders combining these techniques for multi-domain tasks should validate adapter independence empirically instead of relying on routing alone."
score: 43
tier: "notable"
tags: ["models", "research", "coding"]
builder_relevant: true
date: 2026-09-04T04:00:00Z
cluster_size: 1
sources:
  - name: "arXiv cs.LG"
    url: "https://arxiv.org/abs/2609.03150"
    tier: 2
    type: "lab"
takeaways:
  - "Token routing does not prevent adapter interference"
  - "Multi-domain fine-tuning requires empirical validation"
  - "Combine MoE and LoRA with caution"
---
