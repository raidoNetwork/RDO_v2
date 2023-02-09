package state

type State int

/*
Chain state

	Initialized - blockchain state loaded from KV storage
	LocalSynced - blockchain state updated in the SQL storage
	Synced - blockchain is synced with network
	ConnectionHandlersReady - p2p connection handlers attached
	StakePoolInitialized - Stake Pool is initialized with blockchain
	Connected - connected to peers (should precede initial synchronization)
*/
const (
	Initialized State = iota + 1
	LocalSynced
	Synced
	ConnectionHandlersReady
	StakePoolInitialized
	Connected
)
