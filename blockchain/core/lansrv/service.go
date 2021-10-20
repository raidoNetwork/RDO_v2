package lansrv

import (
	"bytes"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"math/rand"
	"rdo_draft/blockchain/consensus"
	"rdo_draft/blockchain/core/rdochain"
	"rdo_draft/blockchain/db"
	"rdo_draft/cmd/blockchain/flags"
	"rdo_draft/proto/prototype"
	"rdo_draft/shared/common"
	"rdo_draft/shared/crypto"
	"rdo_draft/shared/types"
	"sync"
	"time"
)

const (
	// account count
	accountNum = 10

	// initial value of all accounts' balance
	startAmount = 10000000 // 10 * 1e6

	maxFee = 100 // max fee value
	minFee = 1   // min fee value

	txLimit      = 4
	outputsLimit = 5
)

var (
	ErrUtxoSize      = errors.New("UTxO count is 0.")
	ErrAffectedRows  = errors.New("Rows affected are different from expected.")
	ErrInputsRestore = errors.New("Can't restore inputs.")

	genesisTxHash = crypto.Keccak256Hash([]byte("genesis_transaction"))
)

var log = logrus.WithField("prefix", "LanService")

func NewLanSrv(ctx *cli.Context, db db.BlockStorage, outDB db.OutputManager) (*LanSrv, error) {
	accman, err := types.NewAccountManager(ctx)
	if err != nil {
		log.Error("Error creating account manager.", err)
		return nil, err
	}

	// update accman store with key pairs stored in the file
	err = accman.LoadFromDisk()
	if err != nil && !errors.Is(err, types.ErrEmptyStore) {
		log.Error("Error loading accounts.", err)
		return nil, err
	}

	// if keys are not stored create new key pairs and store them
	if errors.Is(err, types.ErrEmptyKeyDir) {
		err = accman.CreatePairs(accountNum)
		if err != nil {
			log.Error("Error creating accounts.", err)
			return nil, err
		}

		// save generated pairs to the disk
		err = accman.StoreToDisk()
		if err != nil {
			log.Error("Error storing accounts to the disk.", err)
			return nil, err
		}
	}

	bc, err := rdochain.NewBlockChain(db, ctx)
	if err != nil {
		log.Error("Error creating blockchain.")
		return nil, err
	}

	statFlag := ctx.Bool(flags.LanSrvStat.Name)
	expStatFlag := ctx.Bool(flags.LanSrvExpStat.Name)

	validator := consensus.NewCryspValidator(bc, outDB, statFlag, expStatFlag)

	srv := &LanSrv{
		ctx:    ctx,
		err:    make(chan error),
		db:     db,
		outDB:  outDB,
		accman: accman,
		bc:     bc,
		validator: validator,

		count:       make(map[string]uint64, accountNum),
		alreadySent: make(map[string]int, accountNum),
		stop:        make(chan struct{}),
		nullBalance: 0,

		// flags
		readTestFlag: ctx.Bool(flags.DBReadTest.Name),
		fullStatFlag: statFlag,

		// inputs
		inputs: map[string][]*types.UTxO{},

		// restore data
		inputsTmp: map[string][]*types.UTxO{},
	}

	err = srv.loadBalances()
	if err != nil {
		return nil, err
	}

	return srv, nil
}

type LanSrv struct {
	ctx  *cli.Context
	err  chan error
	stop chan struct{}

	db    db.BlockStorage  // block storage
	outDB db.OutputManager // utxo storage

	accman *types.AccountManager // key pair storage
	count  map[string]uint64     // accounts tx counter

	alreadySent map[string]int // map of users already sent tx in this block

	bc        *rdochain.BlockChain
	validator consensus.TxValidator // transaction validator

	lastUTxO   uint64 // last utxo id
	readBlocks int    // readed blocks counter

	nullBalance int

	readTestFlag bool
	fullStatFlag bool

	lock sync.RWMutex

	// inputs map[address] -> []*UTxO
	inputs    map[string][]*types.UTxO
	inputsTmp map[string][]*types.UTxO
}

// loadBalances bootstraps service and get balances form UTxO storage
// if user has no genesis tx add it to the user with initial amount
func (s *LanSrv) loadBalances() error {
	log.Info("Bootstrap balances from database.")

	// Generate initial utxo amounts
	var index uint32 = 0
	for addr, _ := range s.accman.GetPairs() {
		addrInBytes := hexToByte(addr)
		if addrInBytes == nil {
			log.Errorf("Can't convert address %s to the slice byte.", addr)
			return errors.Errorf("Can't convert address %s to the slice byte.", addr)
		}

		// find genesis output in DB
		genesisUo, err := s.outDB.FindGenesisOutput(addr)
		if err != nil {
			return errors.Wrap(err, "LoadBalances")
		}

		// create inputs map
		s.inputs[addr] = make([]*types.UTxO, 0)

		// Null genesisUo means that user has no initial balance so add it to him.
		if genesisUo == nil {
			log.Infof("Address %s doesn't have genesis outputs.", addr)

			genesisUo = types.NewUTxO(genesisTxHash, rdochain.GenesisHash, addrInBytes, index, startAmount, rdochain.GenesisBlockNum, common.GenesisTxType)
			genesisUo.TxType = common.GenesisTxType

			id, err := s.outDB.AddOutput(genesisUo)
			if err != nil {
				return errors.Wrap(err, "LoadBalances")
			}

			log.Infof("Add genesis utxo for address: %s with id: %d.", addr, id)

			s.inputs[addr] = append(s.inputs[addr], genesisUo)

			index++

		} else {
			// if genesis unspent
			if genesisUo.Spent == 0 {
				log.Infof("Address %s has unspent genesis output.", addr)

				s.inputs[addr] = append(s.inputs[addr], genesisUo)
			} else {
				// if user has spent genesis select his balance from database
				uoarr, err := s.outDB.FindAllUTxO(addr)
				if err != nil {
					return errors.Wrap(err, "LoadBalances")
				}

				log.Infof("Load all outputs for address %s. Count: %d", addr, len(uoarr))

				for _, uo := range uoarr {
					s.inputs[addr] = append(s.inputs[addr], uo)
				}
			}
		}

		s.count[addr] = 0
	}

	return nil
}

func (s *LanSrv) Start() {
	log.Warn("Start Lan service.")

	go s.generatorLoop()

	if s.readTestFlag {
		go s.readTest()
	}
}

// generatorLoop is main loop of service
func (s *LanSrv) generatorLoop() {
	for {
		select {
		case <-s.stop:
			return
		default:
			start := time.Now()

			num, err := s.genBlockWorker()
			if err != nil {
				log.Errorf("Error in tx generator loop. %s", err.Error())

				if errors.Is(err, ErrUtxoSize) {
					log.Info("Got wrong UTxO count. Reload balances.")
					err = s.loadBalances()

					// if no error try another
					if err == nil {
						continue
					}
				}

				s.err <- err
				return
			}

			if s.fullStatFlag {
				end := time.Since(start)
				log.Infof("Create block in: %s", common.StatFmt(end))
			}

			log.Warnf("Block #%d generated.", num)
		}
	}
}


// genBlockWorker worker for creating one block and store it to the database
func (s *LanSrv) genBlockWorker() (uint64, error) {
	txBatch := make([]*prototype.Transaction, txLimit)
	start := time.Now()

	for i := 0; i < txLimit; i++ {
		startInner := time.Now()

		tx, err := s.genTxWorker(s.bc.GetBlockCount())
		if err != nil {
			log.Errorf("Error creating transaction. %s.", err.Error())

			i--
			continue
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("Generate transaction in %s", common.StatFmt(endInner))
		}

		txBatch[i] = tx
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Generated transaction batch. Count: %d Time: %s", txLimit, common.StatFmt(end))
	}

	// create and store block
	block, err := s.bc.GenerateAndSaveBlock(txBatch)
	if err != nil {
		log.Error("Error creating block.", err)
		return 0, err
	}

	// update SQLite
	start = time.Now()
	err = s.processBlock(block)
	if err != nil {
		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Process block in %s.", common.StatFmt(end))
	}

	// reset already sent map after block creation
	s.alreadySent = make(map[string]int, accountNum)

	return block.Num, nil
}

// genTxWorker creates transaction for block
func (s *LanSrv) genTxWorker(blockNum uint64) (*prototype.Transaction, error) {
	start := time.Now()

	tx, err := s.createTx(blockNum)
	if err != nil {
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Generate tx in %s", common.StatFmt(end))
	}

	start = time.Now()

	err = s.validator.ValidateTransaction(tx)
	if err != nil {
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Validate tx in %s", common.StatFmt(end))
	}

	// save tx data
	from := hex.EncodeToString(tx.Inputs[0].Address)

	// mark user as sender in this block
	s.alreadySent[from] = 1

	// save inputs to the tmp map
	s.inputsTmp[from] = s.inputs[from][:]

	// reset inputs of user
	s.inputs[from] = make([]*types.UTxO, 0)

	if s.fullStatFlag {
		log.Infof("Reset addr %s balance.", from)
	}

	return tx, nil
}

// createTx create tx using in-memory inputs
func (s *LanSrv) createTx(blockNum uint64) (*prototype.Transaction, error) {
	log.Infof("createTx: Start creating tx for block #%d.", blockNum)

	if s.nullBalance >= accountNum {
		return nil, errors.New("All balances are empty.")
	}

	start := time.Now()

	rnd := rand.Intn(accountNum)
	lst := s.accman.GetPairsList()
	pair := lst[rnd]
	from := pair.Address

	// find user that has no transaction in this block yet
	if _, exists := s.alreadySent[from]; exists {
		for exists {
			rnd++

			if rnd == accountNum {
				rnd = 0
			}

			pair = lst[rnd]
			from = pair.Address

			_, exists = s.alreadySent[from]

			log.Infof("Try new transaction sender: %s.", from)
		}
	}

	// if from has no inputs in the memory map return error
	if _, exists := s.inputs[from]; !exists {
		return nil, errors.Errorf("Address %s has no inputs given.", from)
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("createTx: Find sender in %s", common.StatFmt(end))
	}

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
		log.Warnf("Account %s has balanc equal to the minimum fee.", from)
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
	targetAmount := RandUint64(1, balance)

	log.Warnf("Address: %s Balance: %d. Trying to spend: %d.", from, balance, targetAmount)

	change := balance - targetAmount // user change

	if change == 0 {
		return nil, errors.Errorf("Null change.")
	}

	outputCount := rand.Intn(outputsLimit) + 1 // outputs size limit

	var out *prototype.TxOutput

	// create change output
	out = types.NewOutput(hexToByte(from), change)
	opts.Outputs = append(opts.Outputs, out)

	outAmount := targetAmount      // output total balance should be equal to the input balance
	sentOutput := map[string]int{} // list of users who have already received money

	start = time.Now()

	for i := 0; i < outputCount; i++ {
		// all money are spent
		if outAmount == 0 {
			break
		}

		pair = lst[rand.Intn(accountNum)] // get random account receiver
		to := pair.Address

		// check if this output was sent to the address to already
		_, exists := sentOutput[to]

		// if we got the same from go to the cycle start
		if to == from || exists {
			i--
			continue
		}

		// get random amount
		amount := RandUint64(1, outAmount)

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
		out = types.NewOutput(hexToByte(to), amount)
		opts.Outputs = append(opts.Outputs, out)

		sentOutput[to] = 1
		outAmount -= amount
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("createTx: Generate outputs. Count: %d. Time: %s.", len(opts.Outputs), common.StatFmt(end))
	}

	tx, err := types.NewTx(opts)
	if err != nil {
		return nil, err
	}

	realFee := tx.GetRealFee()

	errFeeTooBig := errors.Errorf("Tx can't pay fee. Balance: %d. Value: %d. Fee: %d. Change: %d.", balance, targetAmount, realFee, change)

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

	log.Warnf("Generated tx %s", hex.EncodeToString(tx.Hash))

	return tx, nil
}

// processBlock update SQLite database with given transactions in block.
func (s *LanSrv) processBlock(block *prototype.Block) error {
	start := time.Now()

	blockTx, err := s.outDB.CreateTx()
	if err != nil {
		log.Errorf("processBlock: Error creating DB tx. %s.", err)
		return err
	}

	// update SQLite
	for _, tx := range block.Transactions {
		var from string

		txHash := hex.EncodeToString(tx.Hash)

		startTxInner := time.Now() // whole transaction
		startInner := time.Now()   // inputs or outputs block

		// Only normal Tx has inputs
		if tx.Type == common.NormalTxType {
			from = hex.EncodeToString(tx.Inputs[0].Address)

			// update tx inputs in database
			err := s.processBlockInputs(blockTx, tx)
			if err != nil {
				return err
			}
		} else {
			// for FeeTx and RewardTx
			from = ""
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("processBlock: Update tx %s inputs in %s.", txHash, common.StatFmt(endInner))
		}

		// update outputs
		startInner = time.Now()

		// create tx outputs in the database
		err := s.proccesBlockOutputs(blockTx, tx, from, block.Num)
		if err != nil {
			return err
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("processBlock: Update tx %s outputs in %s.", txHash, common.StatFmt(endInner))
		}

		if s.fullStatFlag {
			endTxInner := time.Since(startTxInner)
			log.Infof("processBlock: Update all transaction %s data in %s.", txHash, common.StatFmt(endTxInner))
		}
	}

	err = s.outDB.CommitTx(blockTx)
	if err != nil {
		log.Error("processBlock: Error committing block transaction.")

		// restore inputs
		s.restoreInputs()

		return err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Update all block inputs, outputs in %s.", common.StatFmt(end))
	}

	// reset inputs store
	s.inputsTmp = make(map[string][]*types.UTxO)

	return nil
}

// processBlockInputs updates all transaction inputs in the database with given DB tx
func (s *LanSrv) processBlockInputs(blockTx int, tx *prototype.Transaction) error {
		var arows int64
	var err error

	// update inputs
	for _, in := range tx.Inputs {
		startIn := time.Now()

		from := hex.EncodeToString(in.Address)
		hash := hex.EncodeToString(in.Hash)

		// doesn't delete genesis from DB
		if bytes.Equal(in.Hash, genesisTxHash) {
			arows, err = s.outDB.SpendGenesis(blockTx, from)
		} else {
			arows, err = s.outDB.SpendOutputWithTx(blockTx, hash, in.Index)
		}

		if err != nil || arows != 1 {
			return s.errorsCheck(arows, blockTx, err)
		}

		if s.fullStatFlag {
			endIn := time.Since(startIn)
			log.Infof("processBlockInputs: Spent one output in %s", common.StatFmt(endIn))
		}
	}

	return nil
}

// processBlockOutputs creates all transaction output in the database with given tx
// and rollback tx if error return
func (s *LanSrv) proccesBlockOutputs(blockTx int, tx *prototype.Transaction, from string, blockNum uint64) error {
	var index uint32 = 0
	var arows int64
	var err error

	for _, out := range tx.Outputs {
		startOut := time.Now()

		uo := types.NewUTxO(tx.Hash, hexToByte(from), out.Address, index, out.Amount, blockNum, int(tx.Type))

		if tx.Type == common.NormalTxType || tx.Type == common.GenesisTxType {
			arows, err = s.outDB.AddOutputWithTx(blockTx, uo)
		} else {
			arows, err = s.outDB.AddNodeOutputWithTx(blockTx, uo)
		}

		if err != nil || arows != 1 {
			return s.errorsCheck(arows, blockTx, err)
		}

		// add output to the map
		to := hex.EncodeToString(uo.To)
		s.inputs[to] = append(s.inputs[to], uo)

		index++

		if s.fullStatFlag {
			endOut := time.Since(startOut)
			log.Infof("processBlockOutputs: Add one output to DB in %s", common.StatFmt(endOut))
			log.Infof("processBlockOutputs: Add %d to addr %s", uo.Amount, to)
		}
	}

	return nil
}

// errorsCheck check error type when updating database and rollback all changes
func (s *LanSrv) errorsCheck(arows int64, blockTx int, err error) error {
	errb := s.outDB.RollbackTx(blockTx)
	if errb != nil {
		log.Errorf("processBlock: Rollback error on addTx: %s.", errb)
	}

	if arows != 1 {
		if err != nil {
			log.Errorf("processBlock: Error: %s.", err.Error())
		}
		log.Errorf("processBlock: Affected rows error. Got: %d.", arows)
		return ErrAffectedRows
	} else {
		log.Errorf("processBlock: %s.", err.Error())
	}

	// restore inputs
	s.restoreInputs()

	return err
}

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

func (s *LanSrv) Stop() error {
	close(s.stop)

	s.showEndStats()

	log.Warn("Stop service.")
	return nil
}

func (s *LanSrv) showEndStats() {
	log.Printf("Blocks: %d Transactions: %d Utxo count: %d.", s.bc.GetBlockCount(), s.bc.GetBlockCount()*txLimit, s.lastUTxO)

	if s.readTestFlag {
		log.Infof("Readed blocks %d.", s.readBlocks)
	}
}

func (s *LanSrv) Status() error {
	select {
	case err := <-s.err:
		return err
	default:
		return nil
	}
}

func (s *LanSrv) readTest() {
	s.readBlocks = 0
	for {
		select {
		case <-s.stop:
			return
		default:
			rand.Seed(time.Now().UnixNano())

			num := rand.Intn(int(s.bc.GetBlockCount())) + 1

			start := time.Now()
			res, err := s.bc.GetBlockByNum(num)
			if err != nil {
				log.Error("Error reading block.", err)
				return
			}

			if res == nil {
				continue
			}

			if s.fullStatFlag {
				end := time.Since(start)
				log.Infof("Read block #%d in %s", num, common.StatFmt(end))
			} else {
				log.Infof("Read block #%d.", num)
			}

			s.readBlocks++
		}
	}
}

func hexToByte(h string) []byte {
	res, err := hex.DecodeString(h)
	if err != nil {
		return nil
	}

	return res
}

func getFee() uint64 {
	/*rand.Seed(time.Now().UnixNano())

	return uint64(rand.Intn(maxFee-minFee) + minFee)*/

	// return fix fee for tests
	return minFee
}

func RandUint64(min, max uint64) uint64 {
	rand.Seed(time.Now().UnixNano())
	rnd := rand.Intn(100) + 1

	res := uint64(rnd)*(max-min)/100 + min

	return res
}