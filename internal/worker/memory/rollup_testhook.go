package memory

// rollupTestHook coordinates deterministic interleaving in tests (B05).
// Production code leaves this nil.
var rollupTestHook *RollupTestHook

type RollupTestHook struct {
	// BypassURILock disables per-URI rollup locks so tests can force bad
	// interleaving (B05 only; never set in production).
	BypassURILock bool
	// BeforeBucketUnchangedQuery runs immediately before the materiality SELECT.
	BeforeBucketUnchangedQuery func(bucketURI string)
	// AfterBucketUnchangedLoad runs after bucketUnchanged reads the active row
	// from the DB and before it compares fingerprints.
	AfterBucketUnchangedLoad func(bucketURI string)
	// BeforeMergeIncumbentCommit runs after mergeIntoIncumbent superseded the
	// incumbent row but before it inserts the new version.
	BeforeMergeIncumbentCommit func(incumbentURI string)
	// AfterMergeIncumbentCommit runs after mergeIntoIncumbent inserted the new row.
	AfterMergeIncumbentCommit func(incumbentURI string)
}
