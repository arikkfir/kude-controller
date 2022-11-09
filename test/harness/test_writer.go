package harness

import "testing"

type testWriter struct {
	T *testing.T
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.T.Helper()
	tw.T.Logf("%s", p)
	return len(p), nil
}
