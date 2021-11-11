package lansrv

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/txpool"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	rmath "github.com/raidoNetwork/RDO_v2/shared/math"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"math/rand"
	"time"
)

var (
	ErrEmptyAll     = errors.New("All balances are empty.")
	ErrSenderChoice = errors.New("Transaction sender choice limit is exceeded.")
)

// genTxWorker creates transaction for block
func (s *LanSrv) genTxWorker(blockNum uint64) error {
	start := time.Now()

	tx, err := s.createTx(blockNum)
	if err != nil {
		return err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Generate tx in %s", common.StatFmt(end))
	}

	start = time.Now()

	// add created tx to the tx pool
	err = s.txpool.SendTx(tx)
	if err != nil {
		return err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Register tx in pool in %s", common.StatFmt(end))
	}

	from := common.BytesToAddress(tx.Inputs[0].Address)

	// mark user as sender in this block
	s.alreadySent[from.Hex()] = 1

	// save inputs to the tmp map
	s.inputsTmp[from.Hex()] = s.inputs[from.Hex()][:]

	// reset inputs of user
	s.inputs[from.Hex()] = make([]*types.UTxO, 0)

	if s.fullStatFlag {
		log.Infof("Reset addr %s balance.", from.Hex())
	}

	return nil
}

// restoreInputs restore inputs from backup map
func (s *LanSrv) restoreInputs() {
	s.lock.Lock()
	for addr, inputs := range s.inputsTmp {
		s.inputs[addr] = inputs[:]

		if s.fullStatFlag {
			log.Infof("restoreInputs: Add inputs count %d to addr %s", len(inputs), addr)
		}
	}
	s.lock.Unlock()
}

// chooseSender choose random tx sender that hasn't sent tx in future block yet
func (s *LanSrv) chooseSender() (string, error) {
	rnd := rand.Intn(common.AccountNum)
	lst := s.accman.GetPairsList()
	pair := lst[rnd]
	from := pair.Address

	// find user that has no transaction in this block yet
	if _, exists := s.alreadySent[from.Hex()]; exists {
		counter := 0
		for exists && counter < common.AccountNum {
			rnd++

			if rnd == common.AccountNum {
				rnd = 0
			}

			pair = lst[rnd]
			from = pair.Address

			_, exists = s.alreadySent[from.Hex()]
			counter++

			log.Infof("Try new transaction sender: %s.", from)
		}

		if counter == common.AccountNum {
			return "", ErrSenderChoice
		}
	}

	// if from has no inputs in the memory map return error
	if _, exists := s.inputs[from.Hex()]; !exists {
		return "", errors.Errorf("Address %s has no inputs given.", from)
	}

	return from.Hex(), nil
}

// generateTxOutputs create tx outputs with given total amount
func (s *LanSrv) generateTxOutputs(targetAmount uint64, from string) []*prototype.TxOutput {
	outAmount := targetAmount
	sentOutput := map[string]int{} // list of users who have already received money
	lst := s.accman.GetPairsList()
	outputCount := rand.Intn(outputsLimit) + 1 // outputs size limit

	var out *prototype.TxOutput
	res := make([]*prototype.TxOutput, 0)

	for i := 0; i < outputCount; i++ {
		// all money are spent
		if outAmount == 0 {
			break
		}

		pair := lst[rand.Intn(common.AccountNum)] // get random account receiver
		to := pair.Address

		// check if this output was sent to the address to already
		_, exists := sentOutput[to.Hex()]

		// if we got the same from go to the cycle start
		if to.Hex() == from || exists {
			i--
			continue
		}

		//amount := uint64(rand.Intn(int(outAmount)) + 1) // get random amount
		amount := rmath.RandUint64(1, outAmount)

		if i != outputCount-1 {
			if amount == outAmount {
				k := uint64(outputCount - i)
				amount = outAmount / k
			}
		} else if outAmount != amount {
			// if it is a last cycle iteration and random amount is not equal to remaining amount (outAmount)
			amount = outAmount
		}

		// create output
		out = types.NewOutput(to.Bytes(), amount, nil)
		res = append(res, out)

		sentOutput[to.Hex()] = 1
		outAmount -= amount
	}

	return res
}

// createTx create tx using in-memory inputs
func (s *LanSrv) createTx(blockNum uint64) (*prototype.Transaction, error) {
	log.Infof("createTx: Start creating tx for block #%d.", blockNum)

	if s.nullBalance >= common.AccountNum {
		return nil, ErrEmptyAll
	}

	start := time.Now()

	from, err := s.chooseSender()
	if err != nil {
		return nil, err
	}

	if s.expStatFlag {
		end := time.Since(start)
		log.Infof("createTx: Find sender in %s", common.StatFmt(end))
	}

	log.Infof("Choose user %s", from)
	userInputs := s.inputs[from]
	inputsArr := make([]*prototype.TxInput, 0, len(userInputs))

	usrKey := s.accman.GetKey(from)

	// count balance
	var balance uint64
	for _, uo := range userInputs {
		balance += uo.Amount

		inputsArr = append(inputsArr, uo.ToInput(usrKey))
	}

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		log.Warnf("Account %s has balance %d.", from, balance)
		s.nullBalance++
		return s.createTx(blockNum)
	}

	// if user has balance equal to the minimum fee find another user
	if balance == minFee {
		log.Warnf("Account %s has balance equal to the minimum fee.", from)
		return s.createTx(blockNum)
	}

	fee := getFee() // get price for 1 byte of tx
	opts := types.TxOptions{
		Fee:    fee,
		Num:    s.count[from],
		Data:   []byte{},
		Type:   common.NormalTxType,
		Inputs: inputsArr,
	}

	// generate random target amount
	targetAmount := rmath.RandUint64(1, balance)

	log.Warnf("Address: %s Balance: %d. Trying to spend: %d.", from, balance, targetAmount)

	change := balance - targetAmount // user change

	if change == 0 {
		return nil, errors.Errorf("Null change.")
	}

	var out *prototype.TxOutput

	// create change output
	addr := common.HexToAddress(from)
	out = types.NewOutput(addr.Bytes(), change, nil)
	opts.Outputs = append(opts.Outputs, out)

	start = time.Now()

	// generate outputs and add it to the tx
	opts.Outputs = append(opts.Outputs, s.generateTxOutputs(targetAmount, from)...)

	if s.expStatFlag {
		end := time.Since(start)
		log.Infof("createTx: Generate outputs. Count: %d. Time: %s.", len(opts.Outputs), common.StatFmt(end))
	}

	tx, err := types.NewTx(opts, usrKey)
	if err != nil {
		return nil, err
	}

	realFee := tx.GetRealFee()

	errFeeTooBig := errors.Errorf("tx can't pay fee. Balance: %d. Value: %d. Fee: %d. Change: %d.", balance, targetAmount, realFee, change)

	if change > realFee {
		change -= realFee
		tx.Outputs[0].Amount = change
	} else if len(tx.Outputs) >= 2 {
		amount := tx.Outputs[1].Amount

		if amount <= realFee {
			return nil, errFeeTooBig
		} else {
			tx.Outputs[1].Amount -= realFee
		}
	} else {
		return nil, errFeeTooBig
	}

	// We change tx outputs so we need to update tx hash.
	// That will make tx hash correct according to changes.
	hash, err := hasher.TxHash(tx)
	if err != nil {
		return nil, err
	}

	tx.Hash = hash[:]

	log.Warnf("Generated tx %s", hash.Hex())

	return tx, nil
}

// generateTxBatch generate and send n transactions to the pool.
func (s *LanSrv) generateTxBatch() error {
	for i := 0; i < common.TxLimitPerBlock; i++ {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			startInner := time.Now()

			// tx worker adds new tx to the tx pool
			err := s.genTxWorker(s.bc.GetBlockCount())
			if err != nil {
				if errors.Is(err, ErrEmptyAll) || errors.Is(err, txpool.ErrPoolClosed) || errors.Is(err, ErrSenderChoice) {
					return err
				}

				log.Errorf("Error creating transaction. %s", err.Error())

				i--
				continue
			}

			if s.fullStatFlag {
				endInner := time.Since(startInner)
				log.Infof("Generate transaction in %s", common.StatFmt(endInner))
			}
		}
	}

	return nil
}

func getFee() uint64 {
	/*rand.Seed(time.Now().UnixNano())

	return uint64(rand.Intn(maxFee-minFee) + minFee)*/

	// return fix fee for tests
	return minFee
}
