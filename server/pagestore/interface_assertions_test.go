package pagestore

// Compile-time assertions that *Store satisfies the Reader and Writer
// interfaces. If either interface drifts in shape, the build fails here
// before tests run — surfaces interface mismatches at the right layer.

var (
	_ Reader = (*Store)(nil)
	_ Writer = (*Store)(nil)
)
