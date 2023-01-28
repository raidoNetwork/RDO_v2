package validator

import "time"

type voting struct {
	approved map[string]struct{}
	rejected map[string]struct{}
	started  time.Time
}
