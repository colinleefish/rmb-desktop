package launchatlogin

// Set enables or disables launching the menu bar app at login.
func Set(enabled bool) error {
	return set(enabled)
}
