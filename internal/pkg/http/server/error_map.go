package server

import (
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"gorm.io/gorm"
)

// registerErrorMappings installs the canonical error translations for the HTTP
// boundary. ctxerrors.Wrap runs registered mappings through errors.Is, so any
// wrapped not-found error — whatever its origin — collapses to
// commerr.ErrNotFound. That lets handlers make ONE not-found decision
// (errors.Is(err, commerr.ErrNotFound)) instead of knowing every
// repo/driver's own not-found sentinel. Idempotent; called from New.
func registerErrorMappings() {
	ctxerrors.MapError(gorm.ErrRecordNotFound, commerr.ErrNotFound)
}
