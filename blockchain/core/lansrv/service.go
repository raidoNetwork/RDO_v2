package lansrv

import (
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
	"sort"
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
	ErrSmallBalance   = errors.New("Balance is too small.")
	ErrUtxoSize       = errors.New("Wrong utxo count.")
	ErrConvertingUtxo = errors.New("Error converting utxo.")

	txHash = crypto.Keccak256Hash([]byte("genesis_transaction"))
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
		//balance: make(map[string]uint64),
		bc: bc,

		count:       make(map[string]uint64, accountNum),
		alreadySent: make(map[string]int, accountNum),
		stop:        make(chan struct{}),
		nullBalance: 0,

		// flags
		readTestFlag: ctx.Bool(flags.DBReadTest.Name),
		fullStatFlag: ctx.Bool(flags.LanSrvStat.Name),
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
}

// loadBalances bootstraps service and get balances form UTxO storage
// if user has no genesis tx add it to the user with initial amount
func (s *LanSrv) loadBalances() error {
	// Generate initial utxo amounts
	var index uint32 = 0

	for addr, _ := range s.accman.GetPairs() {
		addrInBytes := hexToByte(addr)
		if addrInBytes == nil {
			log.Errorf("Can't convert address %s to the slice byte.", addr)
			return errors.Errorf("Can't convert address %s to the slice byte.", addr)
		}

		genesisUo, err := s.outDB.FindGenesisOutput(addr)
		if err != nil {
			return errors.Wrap(err, "LoadBalances")
		}

		// Null genesisUo means that user has not inital balance so add it to him.
		if genesisUo == nil {
			log.Infof("Address %s doesn't have genesis outputs.", addr)

			genesisUo = types.NewUTxO(txHash, rdochain.GenesisHash, addrInBytes, index, startAmount, 1)
			genesisUo.TxType = common.GenesisTxType

			id, err := s.outDB.AddOutput(genesisUo)
			if err != nil {
				return errors.Wrap(err, "LoadBalances")
			}

			log.Infof("Add genesis utxo for address: %s with id: %d.", addr, id)

		}

		index++

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
			num, err := s.genWorker()
			if err != nil {
				log.Error("Error in tx generator loop.", err)
				s.err <- err
				return
			}

			log.Warnf("Block #%d generated.", num)
		}
	}
}

// genWorker worker for creating one block and store it to the database
func (s *LanSrv) genWorker() (uint64, error) {
	txBatch := make([]*prototype.Transaction, txLimit)
	start := time.Now()

	for i := 0; i < txLimit; i++ {
		startInner := time.Now()

		tx, err := s.genTx(s.bc.GetBlockCount())
		if err != nil {
			log.Error("Error creating transaction.", err)
			return 0, err
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

// genTx creates transaction for block
func (s *LanSrv) genTx(blockNum uint64) (*prototype.Transaction, error) {
	log.Info("Start generating transaction")

	if s.nullBalance > accountNum {
		return nil, errors.New("All balances are empty.")
	}

	rnd := rand.Intn(accountNum)
	lst := s.accman.GetPairsList()
	pair := lst[rnd]
	from := pair.Address

	// if from sent transactions before block creation skip it
	if _, exists := s.alreadySent[from]; exists {
		return s.genTx(blockNum)
	}

	start := time.Now()

	utxo, err := s.outDB.FindAllUTxO(from)
	if err != nil {
		return nil, errors.Wrap(err, "genTx")
	}

	utxoSize := len(utxo)

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Read all UTxO of user %s Count: %d Time: %s", from, utxoSize, common.StatFmt(end))
	}

	if utxoSize == 0 {
		return nil, ErrUtxoSize
	}

	// count balance
	var balance uint64
	for _, uo := range utxo {
		balance += uo.Amount
	}

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		log.Warnf("Account %s has balance %d.", from, balance)
		s.nullBalance++
		return s.genTx(blockNum)
	}

	fee := getFee()
	opts := types.TxOptions{
		Fee:  fee,
		Num:  s.count[from],
		Data: []byte{},
		Type: 1,
	}

	opts.Inputs = make([]*prototype.TxInput, 0)

	rand.Seed(time.Now().UnixNano())

	var targetAmount, currentAmount uint64

	currentAmount = 0 // cumulative spent value
	targetAmount = uint64(rand.Intn(int(balance-fee)) + 1)
	inputCount := 0

	log.Warnf("Address: %s Balance: %d", from, targetAmount)

	// list of ids for future spent outputs in database
	spendOutputs := make([]uint64, 0)

	start = time.Now()

	// sort utxo by it's amount from max to min
	sort.SliceStable(utxo, func(i, j int) bool {
		return utxo[i].Amount < utxo[j].Amount
	})

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("UTxO sorted in %s", common.StatFmt(end))
	}

	start = time.Now()

	// generate inputs
	for _, uo := range utxo {
		currentAmount += uo.Amount

		in := uo.ToInput(s.accman.GetKey(from))
		if in == nil {
			return nil, ErrConvertingUtxo
		}

		// update tx inputs
		opts.Inputs = append(opts.Inputs, in)
		inputCount++ // increment output count

		// add output to list for future changes
		spendOutputs = append(spendOutputs, uo.ID)

		if currentAmount >= targetAmount {
			break
		}
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Create inputs Size: %d Time: %s", inputCount, common.StatFmt(end))
	}

	//create outputs
	start = time.Now()

	// set output limit
	outputCount := rand.Intn(outputsLimit) + 1

	// create change output
	change := currentAmount - targetAmount
	if change > 0 {
		s.createOutput(&opts, from, change)
		outputCount--

		log.Infof("User %s has change.", from)
	}

	// for case when we have only change in outputs
	if outputCount == 0 {
		outputCount = 1
	}

	// output total balance should be equal to the input balance
	outAmount := targetAmount

	// list of users who have already received money
	sentOutput := map[string]int{}

	for i := 0; i < outputCount; i++ {
		// all money are spent
		if outAmount == 0 {
			break
		}

		pair = lst[rand.Intn(accountNum)] // get random account
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
		s.createOutput(&opts, to, amount)

		sentOutput[to] = 1
		outAmount -= amount
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Create outputs Size: %d Time: %s", outputCount, common.StatFmt(end))
	}

	tx, err := types.NewTx(opts)
	if err != nil {
		return nil, err
	}

	log.Warnf("Generated tx %s", hex.EncodeToString(tx.Hash))

	start = time.Now()

	txId, err := s.outDB.CreateTx()
	if err != nil {
		log.Error("Error creating spend outputs transaction.")
		return nil, err
	}

	// spend all outputs which was used
	for _, id := range spendOutputs {
		startInner := time.Now()

		err := s.outDB.SpendOutputWithTx(txId, id) // s.outDB.SpendOutput(id)
		if err != nil {
			rollbackErr := s.outDB.RollbackTx(txId)
			log.Error("Spend Rollback error:", rollbackErr)
			return nil, err
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("Spent one output in %s", common.StatFmt(endInner))
		}
	}

	err = s.outDB.CommitTx(txId)
	if err != nil {
		log.Error("Error commiting spend outputs transaction.")
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Spent outputs in %s", common.StatFmt(end))
	}

	// update utxo memory
	start = time.Now()

	txId, err = s.outDB.CreateTx()
	if err != nil {
		log.Error("Error creating add outputs transaction.")
		return nil, err
	}

	var index uint32 = 0
	for _, out := range tx.Outputs {
		// new utxo
		uo := types.NewUTxO(tx.Hash, hexToByte(from), out.Address, index, out.Amount, blockNum)

		startInner := time.Now()

		// add to database
		_, err := s.outDB.AddOutputWithTx(txId, uo)
		if err != nil {
			rollbackErr := s.outDB.RollbackTx(txId)
			log.Error("Add Rollback error:", rollbackErr)

			log.Error("Error creating UTxO", err)
			return nil, err
		}

		if s.fullStatFlag {
			endInner := time.Since(startInner)
			log.Infof("Add one output to DB in %s", common.StatFmt(endInner))
		}

		index++
	}

	err = s.outDB.CommitTx(txId)
	if err != nil {
		log.Error("Error commiting add outputs transaction.")
		return nil, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Create outputs in %s", common.StatFmt(end))
	}

	// update balance
	s.alreadySent[from] = 1
	s.count[from]++

	if s.readTestFlag {
		// check if database writes all data correctly
		id, err := s.outDB.HealthCheck()
		if err != nil {
			log.Error("Error health checking database.", err)
			return nil, err
		}

		s.lastUTxO = id
	}

	return tx, nil
}

func (s *LanSrv) createOutput(opts *types.TxOptions, to string, amount uint64) {
	out := types.NewOutput(hexToByte(to), amount)
	opts.Outputs = append(opts.Outputs, out)
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
