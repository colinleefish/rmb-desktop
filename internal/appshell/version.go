package appshell

// versionPayload mirrors rmbd's GET /api/v1/version response.
type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// versionMatches ports daemon.rs DaemonManager::running_matches_expected.
// The commit check only applies when both sides know a real commit; the Go
// default "dev" (like the Rust "unknown") is treated as unknown so ad-hoc
// dev shells do not recycle daemons in a loop.
func versionMatches(expectedVersion, expectedCommit string, remote versionPayload) bool {
	if expectedVersion != "" && remote.Version != expectedVersion {
		return false
	}
	if isKnownCommit(expectedCommit) && isKnownCommit(remote.Commit) && remote.Commit != expectedCommit {
		return false
	}
	return true
}

// isKnownCommit reports whether a commit stamp is a real commit (not the
// unset/placeholder values).
func isKnownCommit(commit string) bool {
	return commit != "" && commit != "unknown" && commit != "dev"
}
