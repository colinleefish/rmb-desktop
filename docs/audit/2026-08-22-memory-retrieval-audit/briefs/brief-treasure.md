You are a sub-agent in an audit of the user's rmb long-term-memory system. Ground rules:
- The rmb CLI is at ~/.rmb/bin/rmb - ALWAYS use that full path (rmb may not be on PATH).
- STRICTLY READ-ONLY: only rmb search / ls / cat / meta. NEVER run `rmb put`, `rmb hook-submit`, or touch anything under ~/.rmb.
- For every notable CLI call, note roughly how fast it returned.
- Timebox yourself to ~8 minutes of exploration. Be decisive; don't over-verify.
- Deliverable: write a detailed Markdown report to REPORT_PATH (sections: Method, Evidence with exact commands + uris, Findings, Pain points ranked by severity). After writing the file, reply with ONLY the file path, nothing else.

MISSION treasure-hunter: realistic RETRIEVAL stress test. Answer these 8 questions the user might actually ask, using ONLY rmb search/ls/cat/meta. For EACH question log: the exact query string(s) you tried, top-3 results (uri + score if shown), how many follow-up cats were needed, whether you actually got a confident answer, and what annoyed you.
Questions:
1. What happened with the cluster-admin-toolbox? Why was it rejected/removed?
2. What was the BBC deploy k8s parameter split about (bbc-deploy-k8s-param-split)?
3. What is the user's A-share (A股) first-board (首板) strategy entry rule?
4. What is the user's photo workflow (HEIC, Nikon Z30)?
5. Why did rmbd choose sqlite? And separately, what was the duckdb-to-postgres move for PBP?
6. How does the user SSH through the jump.hs99.vip bastion?
7. (vague Chinese) 那个删除标签的问题最后怎么解决的？
8. (ambiguous) starlink openresty - the user wants the dynamic DNS solution details.
Then try 2 queries of your own invention (one typo'd, one Chinese-vs-English mismatch) and report how retrieval behaves.
