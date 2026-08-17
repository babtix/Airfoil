# Airfoil

**High-velocity, low-drag AI intelligence pipeline.**

Airfoil is a tactical news aggregator designed to cut through the noise of the oversaturated AI ecosystem. It is not a standard RSS reader; it is an automated reconnaissance agent that strips away marketing fluff, clusters redundant coverage, and delivers pure technical signal.

### Core Mechanics

* **Radar & Ingest:** Scans high-tier primary sources (AI labs, arXiv, Hacker News, HuggingFace) while ignoring low-tier press.
* **Clustering & Deduplication:** Groups multiple outlets covering the exact same release into a single, unified "Payload" to prevent feed clutter.
* **Telemetry & Scoring:** Ranks clusters based on source authority, community validation, and time-decay, filtering out clickbait.
* **LLM Summarization:** Processes the highest-scoring clusters to generate dense, zero-slop summaries of the core technical takeaways.

### Technical Architecture

* **The Engine (Backend):** A single, cross-compiled **Go CLI Agent** that executes the entire pipeline (ingest, embed, cluster, score, summarize, and write) with zero persistent server costs.
* **The Chassis (Frontend):** An **Astro** static site that consumes the Go agent's output as pure Markdown files, delivering maximum SEO, zero database overhead, and instant load times.
* **Avionics (Search):** **Pagefind** integration for lightning-fast, fully client-side static search capabilities.
