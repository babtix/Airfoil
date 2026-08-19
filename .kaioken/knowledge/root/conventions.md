Go: no globals, no `init()`; config passed explicitly; pure functions (`normalize`, `cluster`, `score`) require table-driven tests in `internal/`
Env only: copy `.env.example` to `.env`; never commit `.env`; CI uses GitHub Secrets
Idempotency: second agent run creates zero duplicates (state tracks seen IDs per day)
Content limits: max 300 chars source text stored; max one quote per source (<15 words); summaries are original LLM prose
Zero client JS except `/search` page
Design tokens: `site/src/styles/tokens.css` referenced everywhere, no inline values
Dense layout: ~25 stories per desktop screen on `/feed`
Agent commits: `content: YYYY-MM-DD — N stories` — only commits touching `data/` or `site/src/content/`
Validation gate: failed validation aborts before commit; never push a broken site
Never commit `data/cache/` — embeddings are regenerable and huge
