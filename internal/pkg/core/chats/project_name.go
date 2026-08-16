package chats

import (
	"strings"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

const MaxProjectNameRunes = 80

func validatedProjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ctxerrors.Wrap(
			commerr.ErrInvalidArgument,
			"project name must not be empty",
		)
	}

	if len([]rune(name)) > MaxProjectNameRunes {
		return "", ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"project name exceeds %d characters",
			MaxProjectNameRunes,
		)
	}

	return name, nil
}
