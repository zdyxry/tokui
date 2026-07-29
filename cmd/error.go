package cmd

// CLIError marks an expected user error (bad flags, not a git repository,
// missing tools, conflicting modes, ...). Execute prints it plainly instead
// of the crash report (stack trace + issue prompt) reserved for unexpected
// failures.
type CLIError struct {
	ctxErr error
}

func NewCLIError(err error) *CLIError {
	return &CLIError{ctxErr: err}
}

func (err *CLIError) Error() string {
	return err.ctxErr.Error()
}

func (err *CLIError) Unwrap() error {
	return err.ctxErr
}

// runError marks an error that escaped runApp, so Execute can tell command
// failures apart from cobra flag parsing/validation errors (which are also
// expected user errors and must not render as crash reports).
type runError struct {
	err error
}

func (err *runError) Error() string {
	return err.err.Error()
}

func (err *runError) Unwrap() error {
	return err.err
}
