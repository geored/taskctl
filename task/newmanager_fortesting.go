package task

// NewManagerForTesting creates a Manager whose backing file is at absPath.
// absPath may be absolute; the relative-path restriction enforced by
// NewManager is intentionally bypassed.
//
// THIS FUNCTION EXISTS SOLELY TO SUPPORT TEST ISOLATION. It must not be
// called from any non-test code path. Production code must always use
// NewManager so that the security policy (relative-path-only) is enforced.
func NewManagerForTesting(absPath string) *Manager {
	return &Manager{filePath: absPath}
}
