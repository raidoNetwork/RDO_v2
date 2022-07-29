package state

type State int

const (
	Initialized State = iota + 1
	LocalSynced
	Synced
)