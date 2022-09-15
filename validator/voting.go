package validator

import "time"

type voting struct {
	approved int
	rejected int
	started time.Time
}
