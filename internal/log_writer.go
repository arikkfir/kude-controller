package internal

import "github.com/go-logr/logr"

type logWriter struct {
	UseError bool
	l        logr.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	if w.UseError {
		w.l.Error(nil, string(p))
	} else {
		w.l.Info(string(p))
	}
	return len(p), nil
}
