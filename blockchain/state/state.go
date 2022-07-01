package state

type State int

const (
	LocalSynced State = iota + 1
	Synced
	ForgeFailed
	Syncing
)