package eio

// This is a wrapper for the errors internal to engine.io.
//
// If you see this error, this means that the problem is
// neither a network error, nor an error caused by you, but
// the source of the error is engine.io. Open an issue on GitHub.
type InternalError struct {
	err error
}

func (e InternalError) Error() string {
	return "eio: internal error: " + e.err.Error()
}

func (e InternalError) Unwrap() error { return e.err }

func wrapInternalError(err error) *InternalError {
	return &InternalError{err: err}
}
