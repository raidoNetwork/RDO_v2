package consensus

import "github.com/pkg/errors"

var (
	ErrNoStakers      = errors.New("No stake deposit is registered.")
	ErrUtxoSize       = errors.New("UTxO count is 0.")
	ErrSmallFee       = errors.New("Too small fee price.")
	ErrLowStakeAmount = errors.New("Too small stake amount.")
	ErrStakeLimit     = errors.New("All stake slots are filled.")
	ErrBadNonce       = errors.New("Wrong transaction nonce.")
	ErrBadTxType      = errors.New("Undefined tx type.")
	ErrBadInputOwner  = errors.New("Bad input owner")

	ErrKnownBlock     = errors.New("Block already exists in the database")
)
