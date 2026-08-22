You are a sub-agent in an audit of the user's rmb long-term-memory system. Ground rules:
- The rmb CLI is at ~/.rmb/bin/rmb - ALWAYS use that full path (rmb may not be on PATH).
- STRICTLY READ-ONLY: only rmb search / ls / cat / meta. NEVER run `rmb put`, `rmb hook-submit`, or touch anything under ~/.rmb.
- For every notable CLI call, note roughly how fast it returned.
- Timebox yourself to ~8 minutes of exploration. Be decisive; don't over-verify.
- Deliverable: write a detailed Markdown report to REPORT_PATH (sections: Method, Evidence with exact commands + uris, Findings, Pain points ranked by severity). After writing the file, reply with ONLY the file path, nothing else.

MISSION fact-checker: CONSISTENCY and QUALITY audit.
1. Randomly sample 12 entities and 12 preferences (use ls | shuf or similar). cat each one fully.
2. Look for: (a) DUPLICATES - same fact under two different uris; (b) CONTRADICTIONS - e.g. old vs new hardware/hostnames/emails/roles inside entities vs what the profile says; (c) STALENESS - anything referencing outdated state (old Macs, old projects) without a date; (d) MISCATEGORIZATION - preferences that are actually facts, entities that are actually events.
3. Cross-reference integrity: pick 10 uris mentioned inside memory bodies (as text like rmb://... or implied links) and cat them - do they resolve?
4. Query robustness: take 3 facts you found and try to retrieve each with (a) the Chinese phrasing, (b) the English phrasing, (c) a typo'd version. Do all three retrieve the same memory? Report ranking differences.
