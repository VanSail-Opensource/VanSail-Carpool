package backup

import "errors"

// ErrRestoreConflict protects archives referenced by historical unfinished recovery.
var ErrRestoreConflict = errors.New("backup is protected by recovery history")
