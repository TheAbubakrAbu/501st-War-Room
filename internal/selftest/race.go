//go:build race

package selftest

// The race detector slows drawing several times over, so the draw-time
// check only runs in normal builds (the deployed binary and CI's self-test).
const raceBuild = true
