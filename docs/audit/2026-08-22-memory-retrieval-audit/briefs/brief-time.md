You are a sub-agent in an audit of the user's rmb long-term-memory system. Ground rules:
- The rmb CLI is at ~/.rmb/bin/rmb - ALWAYS use that full path (rmb may not be on PATH).
- STRICTLY READ-ONLY: only rmb search / ls / cat / meta. NEVER run `rmb put`, `rmb hook-submit`, or touch anything under ~/.rmb.
- For every notable CLI call, note roughly how fast it returned.
- Timebox yourself to ~8 minutes of exploration. Be decisive; don't over-verify.
- Deliverable: write a detailed Markdown report to REPORT_PATH (sections: Method, Evidence with exact commands + uris, Findings, Pain points ranked by severity). After writing the file, reply with ONLY the file path, nothing else.

MISSION time-traveler: TIMELINE and RECENCY tests.
1. Verify `ls rmb://events/` is truly ordered by updated_at DESC: run `meta` on 10 samples down the list. Any inversions?
2. The ls cap (~200): try hard to see older events beyond the cap (pagination? scope? daemon API?). How many events exist in total vs visible?
3. Date-less slugs: sqlite-choice, rmbd-sqlite-architecture, duckdb-to-postgres-pbp have no date prefix. Can you determine WHEN they happened? Is discoverability-by-time broken for them?
4. "What did I do most recently?" test: compare `rmb search "recent work"` top-10 vs `ls rmb://events/ | head`. Does search surface the newest events at all? Quantify: of the top-10 search hits, how many are from the last 7 days?
5. Events are documented as "immutable dated decisions" - check meta on 10 events: any created_at != updated_at divergence? What does an updated event mean?
6. Scenes: ls rmb://scenes/ - do scene dates correlate with event dates? Any sessions with scenes but no events or vice versa (sample a few session ids)?
