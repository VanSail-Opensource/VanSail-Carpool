package xnode

// Compilation diagnostics can include administrator documents or private peer
// fields. Default errors/logs expose only the stage; explicit validation callers
// can inspect a bounded detail without serializing or logging it implicitly.
type CompilationError struct {
	Stage  string
	detail string
}

func (e *CompilationError) Error() string            { return ErrInvalid.Error() + ": " + e.Stage }
func (e *CompilationError) Unwrap() error            { return ErrInvalid }
func (e *CompilationError) ValidationDetail() string { return e.detail }
func rejectedCompilation(stage, detail string) error {
	runes := []rune(detail)
	if len(runes) > 2048 {
		runes = runes[:2048]
	}
	return &CompilationError{Stage: stage, detail: string(runes)}
}
