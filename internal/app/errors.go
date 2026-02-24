package app

import "strings"

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func WithCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return codedError{code: code, err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	if ce, ok := err.(codedError); ok {
		return ce.code
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unknown command") || strings.Contains(msg, "usage"):
		return ExitUsage
	case strings.Contains(msg, "config") || strings.Contains(msg, "paw.toml"):
		return ExitConfig
	case strings.Contains(msg, "conflict"):
		return ExitConflict
	case strings.Contains(msg, "hook"):
		return ExitHookFailure
	case strings.Contains(msg, "rollback"):
		return ExitRollbackError
	default:
		return ExitSystem
	}
}
