package vm

import "errors"

const (
	ErrPortRange      = "port-range"
	ErrPortRequired   = "port-required"
	ErrProject        = "project"
	ErrShare          = "share"
	ErrSaveSettings   = "save-settings"
	ErrPrepare        = "prepare"
	ErrMountsChanged  = "mounts-changed"
	ErrDesktop        = "desktop"
	ErrDesktopRestart = "desktop-restart"
	ErrCommand        = "command"
	ErrCreateSession  = "create-session"
	ErrStop           = "stop"
	ErrKillSession    = "kill-session"
	ErrRoute          = "route"
	ErrSnapshot       = "snapshot"
)

type operationError struct {
	code string
	err  error
}

func (e *operationError) Error() string { return e.err.Error() }
func (e *operationError) Unwrap() error { return e.err }

// ErrorCode returns the user-facing reason, leaving the wrapped error available for diagnostics.
func ErrorCode(err error) string {
	var op *operationError
	if errors.As(err, &op) {
		return op.code
	}
	return ""
}

func withCode(code string, err error) error {
	if err == nil || ErrorCode(err) != "" {
		return err
	}
	return &operationError{code: code, err: err}
}
