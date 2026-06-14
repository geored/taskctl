package task

// newManagerFromPath is an unexported alias used by tests within the task
// package. It delegates to the exported NewManagerForTesting so there is a
// single implementation. Defined here (_test.go) so the unexported name is
// only reachable from within the task package's test binary.
func newManagerFromPath(absPath string) *Manager {
	return NewManagerForTesting(absPath)
}
