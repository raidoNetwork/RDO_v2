package lansrv

import (
	"bytes"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"math/rand"
	"rdo_draft/blockchain/core/rdochain"
	"rdo_draft/blockchain/db"
	"rdo_draft/cmd/blockchain/flags"
	"rdo_draft/proto/prototype"
	"rdo_draft/shared/common"
	"rdo_draft/shared/crypto"
	"rdo_draft/shared/types"
	"strconv"
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

	srv := &LanSrv{
		ctx:    ctx,
		err:    make(chan error),
		db:     db,
		outDB:  outDB,
		accman: accman,
		bc:     bc,

		count:       make(map[string]uint64, accountNum),
		alreadySent: make(map[string]int, accountNum),
		stop:        make(chan struct{}),
		nullBalance: 0,

		// flags
		readTestFlag: ctx.Bool(flags.DBReadTest.Name),
		fullStatFlag: ctx.Bool(flags.LanSrvStat.Name),

		// inputs
		inputs: map[string][]*types.UTxO{},

		// restore data
		inputsTmp: []*types.UTxO{},
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

	bc *rdochain.BlockChain

	lastUTxO   uint64 // last utxo id
	readBlocks int    // readed blocks counter

	nullBalance int

	readTestFlag bool
	fullStatFlag bool

	lock sync.RWMutex

	// inputs map[address] -> []*UTxO
	inputs    map[string][]*types.UTxO
	inputsTmp []*types.UTxO
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

			genesisUo = types.NewUTxO(genesisTxHash, rdochain.GenesisHash, addrInBytes, index, startAmount, 1)
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
				log.Error("Error in tx generator loop.", err)
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
		log.Infof("Generate transactions for block. Count: %d Time: %s", txLimit, common.StatFmt(end))
	}

	// create and store block
	block, err := s.bc.GenerateAndSaveBlock(txBatch)
	if err != nil {
		log.Error("Error creating block.", err)
		return 0, err
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

	err = s.validateTx(tx)
	if err != nil {
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Validate tx in %s", common.StatFmt(end))
	}

	start = time.Now()

	err = s.processTx(tx, blockNum)
	if err != nil {
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genTxWorker: Prccess tx in %s", common.StatFmt(end))
	}

	return tx, nil
}

// createTx create tx using in-memory inputs
func (s *LanSrv) createTx(blockNum uint64) (*prototype.Transaction, error) {
	log.Infof("createTx: Start creating tx for block #%d.", blockNum)

	if s.nullBalance == accountNum {
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

	start = time.Now()

	usrKey := s.accman.GetKey(from)

	// count balance
	var balance uint64
	for _, uo := range userInputs {
		balance += uo.Amount

		inputsArr = append(inputsArr, uo.ToInput(usrKey))
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("createTx: Count balance in %s", common.StatFmt(end))
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

	fee := getFee()
	opts := types.TxOptions{
		Fee:    fee,
		Num:    s.count[from],
		Data:   []byte{},
		Type:   common.NormalTxType,
		Inputs: inputsArr,
	}

	start = time.Now()

	// maximum value user can spent
	effectiveBalance := balance - fee

	// if we got negative or zero effective balance
	// find another fee and try one more time
	for effectiveBalance <= 0 {
		fee = getFee()
		effectiveBalance = balance - fee
	}

	// debug stat
	end := time.Since(start)
	log.Debugf("createTx: Find effective balance in %s", common.StatFmt(end))

	// generate random target amount
	rand.Seed(time.Now().UnixNano())
	targetAmount := uint64(rand.Intn(int(effectiveBalance)) + 1)

	log.Warnf("Address: %s Balance: %d. Trying to spend: %d.", from, balance, targetAmount)

	outputCount := rand.Intn(outputsLimit) + 1 // outputs size limit
	change := effectiveBalance - targetAmount  // user change

	// check something strange
	if change < 0 {
		return nil, errors.Errorf("User %s has negative change: %d. Effective balance: %d. Target amount: %d.", from, change, effectiveBalance, targetAmount)
	}

	var out *prototype.TxOutput

	// create change output
	if change > 0 {
		out = types.NewOutput(hexToByte(from), change)
		opts.Outputs = append(opts.Outputs, out)

		log.Infof("Account %s has change: %d.", from, change)
	}

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

		amount := uint64(rand.Intn(int(outAmount)) + 1) // get random amount

		// if it is a last cycle iteration and random amount is not equal to remaining amount (outAmount)
		if i == outputCount-1 && outAmount != amount {
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

	log.Warnf("Generated tx %s", hex.EncodeToString(tx.Hash))

	return tx, nil
}

// validateTx check if tx is correct
func (s *LanSrv) validateTx(tx *prototype.Transaction) error {
	if len(tx.Inputs) == 0 {
		return errors.Errorf("Empty tx inputs.")
	}

	if len(tx.Outputs) == 0 {
		return errors.Errorf("Empty tx outputs.")
	}

	start := time.Now()

	// check that inputs and outputs balance with fee are equal
	var txBalance uint64 = 0
	for _, in := range tx.Inputs {
		txBalance += in.Amount

		// TODO add signature validation here
	}

	for _, out := range tx.Outputs {
		txBalance -= out.Amount
	}

	txBalance -= tx.Fee

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("validateTx: Check Tx balance in %s.", common.StatFmt(end))
	}

	if txBalance != 0 {
		return errors.Errorf("Tx balance is inconsistent. Mismatch is %d.", txBalance)
	}

	start = time.Now()

	// FIXME in tests all inputs have the same address but in real it can be different
	from := hex.EncodeToString(tx.Inputs[0].Address)
	usrKey := s.accman.GetKey(from)

	// get user inputs from DB
	utxo, err := s.outDB.FindAllUTxO(from)
	if err != nil {
		return errors.Wrap(err, "validateTx")
	}

	utxoSize := len(utxo)

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("validateTx: Read all UTxO of user %s Count: %d Time: %s", from, utxoSize, common.StatFmt(end))
	}

	inputsSize := len(tx.Inputs)

	if utxoSize != inputsSize {
		return errors.Errorf("validateTx: Inputs size mismatch: real - %d given - %d.", utxoSize, inputsSize)
	}

	if utxoSize == 0 {
		return ErrUtxoSize
	}

	// create spentOutputs map
	spentOutputsMap := map[string]*prototype.TxInput{}

	start = time.Now()

	// count balance and create spent map
	var balance uint64
	var key, hash, indexStr string
	for _, uo := range utxo {
		hash = hex.EncodeToString(uo.Hash)
		indexStr = strconv.Itoa(int(uo.Index))
		key = hash + "_" + indexStr

		// fill map with outputs from db
		spentOutputsMap[key] = uo.ToInput(usrKey)
		balance += uo.Amount
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("validateTx: Count balance in %s", common.StatFmt(end))
	}

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		return errors.Errorf("Account %s has balance 0.", from)
	}

	alreadySpent := map[string]int{}

	// Inputs verification
	for _, in := range tx.Inputs {
		hash = hex.EncodeToString(in.Hash)
		indexStr = strconv.Itoa(int(in.Index))
		key = hash + "_" + indexStr

		dbInput, exists := spentOutputsMap[key]
		if !exists {
			return errors.Errorf("User %s gave undefined output with key: %s.", from, key)
		}

		if alreadySpent[key] == 1 {
			return errors.Errorf("User %s try to spend output twice with key: %s.", from, key)
		}

		if !bytes.Equal(dbInput.Hash, in.Hash) {
			return errors.Errorf("Hash mismatch with key %s. Given %s. Expected %s.", key, hash, hex.EncodeToString(dbInput.Hash))
		}

		if in.Index != dbInput.Index {
			return errors.Errorf("Index mismatch with key %s. Given %d. Expected %d.", key, in.Index, dbInput.Index)
		}

		if in.Amount != dbInput.Amount {
			return errors.Errorf("Amount mismatch with key: %s. Given %d. Expected %d.", key, in.Amount, dbInput.Amount)
		}

		// TODO check negative amount case

		// mark output as already spent
		alreadySpent[key] = 1
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("validateTx: Inputs verification. Count: %d. Time: %s.", inputsSize, common.StatFmt(end))
	}

	start = time.Now()

	//Check that all outputs are spent
	for _, isSpent := range alreadySpent {
		if isSpent != 1 {
			return errors.Errorf("Unspent output of user %s with key %s.", from, key)
		}
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("validateTx: Verify all inputs are lock for spent in %s.", common.StatFmt(end))
	}

	log.Warnf("Validated tx %s", hex.EncodeToString(tx.Hash))

	return nil
}

// processTx add tx changes to the databases
func (s *LanSrv) processTx(tx *prototype.Transaction, blockNum uint64) error {
	// FIXME in tests all inputs have the same address but in real it can be different
	from := hex.EncodeToString(tx.Inputs[0].Address)

	start := time.Now()

	txId, err := s.outDB.CreateTx()
	if err != nil {
		log.Error("processTx: Error creating database transaction.")
		return err
	}

	var affectedRows int64

	// spend all outputs which was used
	for _, in := range tx.Inputs {
		startInner := time.Now()

		// doesn't delete genesis from DB
		if bytes.Equal(in.Hash, genesisTxHash) {
			affectedRows, err = s.outDB.SpendGenesis(txId, from)
		} else {
			affectedRows, err = s.outDB.SpendOutputWithTx(txId, hex.EncodeToString(in.Hash), in.Index)
		}

		if err != nil {
			log.Errorf("processTx: Error spending output: %s.", err.Error())

			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("processTx: Rollback error: %s.", errb)
			}

			return err
		}

		if affectedRows != 1 {
			log.Errorf("Affected rows %d while executing query for spending.", affectedRows)

			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("processTx: Rollback error: %s.", errb)
			}

			return ErrAffectedRows
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("processTx: Spent one output in %s", common.StatFmt(endInner))
		}
	}

	err = s.outDB.CommitTx(txId)
	if err != nil {
		log.Error("processTx: Error committing spend outputs transaction.")

		errb := s.outDB.RollbackTx(txId)
		if errb != nil {
			log.Errorf("processTx: Rollback error: %s.", errb)
		}

		return err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("processTx: Spent outputs in %s", common.StatFmt(end))
	}

	// store sender inputs for future
	s.inputsTmp = make([]*types.UTxO, 0, len(s.inputs[from]))
	copy(s.inputsTmp, s.inputs[from])

	// reset sender inputs
	s.inputs[from] = make([]*types.UTxO, 0)

	// update utxo memory
	start = time.Now()

	txId, err = s.outDB.CreateTx()
	if err != nil {
		log.Error("processTx: Error creating add outputs transaction.")
		return err
	}

	var index uint32 = 0
	for _, out := range tx.Outputs {
		// new utxo
		uo := types.NewUTxO(tx.Hash, hexToByte(from), out.Address, index, out.Amount, blockNum)

		startInner := time.Now()

		// add to database
		affectedRows, err = s.outDB.AddOutputWithTx(txId, uo)
		if err != nil {
			log.Errorf("processTx: Error adding output: %s.", err.Error())

			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("processTx: Rollback error: %s.", errb)
			}

			// try to restore inputs of sender in database
			errb = s.restoreInputs()
			if errb != nil {
				log.Panicf("Can't restore inputs of user %s. %s.", from, errb.Error())
			}

			return err
		}

		if affectedRows != 1 {
			log.Errorf("Affected rows %d while executing query for adding.", affectedRows)

			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("processTx: Rollback error: %s.", errb)
			}

			// try to restore inputs of sender in database
			errb = s.restoreInputs()
			if errb != nil {
				log.Panicf("Can't restore inputs of user %s. %s.", from, errb.Error())
			}

			return ErrAffectedRows
		}

		// add output to the map
		to := hex.EncodeToString(uo.To)
		s.inputs[to] = append(s.inputs[to], uo)

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("Add one output to DB in %s", common.StatFmt(endInner))
		}

		index++
	}

	err = s.outDB.CommitTx(txId)
	if err != nil {
		log.Error("processTx: Error committing add outputs transaction.")

		errb := s.outDB.RollbackTx(txId)
		if errb != nil {
			log.Errorf("processTx: Rollback error: %s.", errb)
		}

		return err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Create outputs in %s", common.StatFmt(end))
	}

	// update stats
	s.alreadySent[from] = 1
	s.count[from]++

	log.Warnf("Processed tx %s", hex.EncodeToString(tx.Hash))

	return nil
}

func (s *LanSrv) restoreInputs() error {
	log.Info("Start restoring inputs")

	start := time.Now()

	txId, err := s.outDB.CreateTx()
	if err != nil {
		log.Error("restoreInputs: Error creating add outputs transaction.")
		return err
	}

	var affectedRows int64
	for _, uo := range s.inputsTmp {
		startInner := time.Now()

		// restore input in the database
		affectedRows, err = s.outDB.AddOutputWithTx(txId, uo)
		if err != nil {
			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("restoreInputs: Rollback error: %s.", errb)
			}

			return err
		}

		if affectedRows != 1 {
			log.Errorf("restoreInputs: Affected rows %d while executing query for adding.", affectedRows)

			errb := s.outDB.RollbackTx(txId)
			if errb != nil {
				log.Errorf("restoreInputs: Rollback error: %s.", errb)
			}

			return ErrAffectedRows
		}

		// add output to the map
		to := hex.EncodeToString(uo.To)
		s.inputs[to] = append(s.inputs[to], uo)

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("Restore one output in %s", common.StatFmt(endInner))
		}
	}

	err = s.outDB.CommitTx(txId)
	if err != nil {
		log.Error("restoreInputs: Error committing add outputs transaction.")

		errb := s.outDB.RollbackTx(txId)
		if errb != nil {
			log.Errorf("restoreInputs: Rollback error: %s.", errb)
		}

		return err
	}

	// reset inputs diff
	s.inputsTmp = make([]*types.UTxO, 0)

	log.Info("Inputs successfully restore in the database.")

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Restore all outputs in %s", common.StatFmt(end))
	}

	return nil
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
	rand.Seed(time.Now().UnixNano())

	return uint64(rand.Intn(maxFee-minFee) + minFee)
}
