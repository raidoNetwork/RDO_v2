package params

import (
	"time"
)

// RDOBlockChainConfig contains constant configs for node to participate in raido blockchain.
type RDOBlockChainConfig struct {
	// Initial value constants.
	ZeroHash [32]byte // ZeroHash is used to represent a zeroed out 32 byte array.

	// RDO constants.
	DefaultBufferSize        int           // DefaultBufferSize for channels across the RDO repository.
	RPCSyncCheck             time.Duration // Number of seconds to query the sync service, to find out if the node is synced or not.
	EmptySignature           [96]byte      // EmptySignature is used to represent a zeroed out BLS Signature.
	DefaultPageSize          int           // DefaultPageSize defines the default page size for RPC server request.
	MaxPeersToSync           int           // MaxPeersToSync describes the limit for number of peers in round robin sync.
	GenesisCountdownInterval time.Duration // How often to log the countdown until the genesis time is reached.
}
