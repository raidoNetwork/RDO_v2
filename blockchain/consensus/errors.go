package consensus

import "github.com/pkg/errors"

var (
	ErrNoStakers      = errors.New("No stake deposit is registered.")
)

