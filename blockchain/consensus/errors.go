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
	ErrEmptyInputs    = errors.New("No inputs.")
	ErrEmptyOutputs   = errors.New("No outputs.")
)
