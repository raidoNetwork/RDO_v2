package lansrv

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/miner"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/txpool"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	rmath "github.com/raidoNetwork/RDO_v2/shared/math"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"sync"
	"time"
)

const (
	maxFee = 100 // max fee value
	minFee = 1   // min fee value

	outputsLimit = 5
)

var (
	ErrUtxoSize = errors.New("UTxO count is 0.")
)

// system hex addr bb652b92498c3be9af648d37985095b6e17200cd0913d95bd383d572de1f3886

var log = logrus.WithField("prefix", "LanService")

func NewLanSrv(cliCtx *cli.Context, db db.BlockStorage, outDB db.OutputStorage) (*LanSrv, error) {
	accman, err := prepareAccounts(cliCtx)
	if err != nil {
		log.Errorf("Error generating accounts. %s", err)
	}

	bc, err := rdochain.NewBlockChain(db, cliCtx)
	if err != nil {
		log.Error("Error creating blockchain.")
		return nil, err
	}

	statFlag := cliCtx.Bool(flags.SrvStat.Name)
	expStatFlag := cliCtx.Bool(flags.SrvExpStat.Name)

	// output manager
	outm := rdochain.NewOutputManager(bc, outDB, &rdochain.OutputManagerConfig{
		ShowStat: statFlag,
	})

	// sync database data
	err = outm.SyncData()
	if err != nil {
		return nil, err
	}

	// check DB consistency
	err = outm.CheckBalance()
	if err != nil {
		return nil, err
	}

	// new block and tx validator
	validator := consensus.NewCryspValidator(bc, outm, statFlag, expStatFlag)

	// new tx pool
	txPool := txpool.NewTxPool(validator)

	// new block miner
	forger := miner.NewMiner(bc, validator, txPool, outm, &miner.MinerConfig{
		ShowStat:     statFlag,
		ShowFullStat: expStatFlag,
	})

	ctx, finish := context.WithCancel(context.Background())

	srv := &LanSrv{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		err:        make(chan error),
		outm:       outm,
		accman:     accman,
		bc:         bc,
		txpool:     txPool,
		miner:      forger,

		count:       make(map[string]uint64, common.AccountNum),
		alreadySent: make(map[string]int, common.AccountNum),
		stop:        make(chan struct{}),
		nullBalance: 0,

		// flags
		readTestFlag: cliCtx.Bool(flags.DBReadTest.Name),
		fullStatFlag: statFlag,
		expStatFlag:  expStatFlag,

		// inputs
		inputs: map[string][]*types.UTxO{},

		// restore data
		inputsTmp: map[string][]*types.UTxO{},

		blockStat: map[string]int64{},
	}

	srv.initBlockStat()

	err = srv.loadBalances()
	if err != nil {
		return nil, err
	}

	return srv, nil
}

func prepareAccounts(ctx *cli.Context) (*types.AccountManager, error) {
	accman, err := types.NewAccountManager(ctx)
	if err != nil {
		log.Error("Error creating account manager.", err)
		return nil, err
	}

	// update accman store with key pairs stored in the file
	err = accman.LoadFromDisk()
	if err != nil && !errors.Is(err, types.ErrEmptyKeyDir) {
		log.Error("Error loading accounts.", err)
		return nil, err
	}

	// if keys are not stored create new key pairs and store them
	if errors.Is(err, types.ErrEmptyKeyDir) {
		err = accman.CreatePairs(common.AccountNum)
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

	log.Warn("Account list is ready.")

	return accman, nil
}

type LanSrv struct {
	cliCtx     *cli.Context
	ctx        context.Context
	cancelFunc context.CancelFunc
	err        chan error
	stop       chan struct{}

	outm *rdochain.OutputManager // output manager

	accman *types.AccountManager // key pair storage
	count  map[string]uint64     // accounts tx counter

	alreadySent map[string]int // map of users already sent tx in this block

	bc     *rdochain.BlockChain
	miner  *miner.Miner
	txpool *txpool.TxPool

	lastUTxO   uint64 // last utxo id
	readBlocks int    // readed blocks counter

	nullBalance int

	// flags
	readTestFlag bool
	fullStatFlag bool
	expStatFlag  bool

	lock sync.RWMutex

	// inputs map[address] -> []*UTxO
	inputs    map[string][]*types.UTxO
	inputsTmp map[string][]*types.UTxO

	blockStat map[string]int64
}

// loadBalances bootstraps service and get balances form UTxO storage
func (s *LanSrv) loadBalances() error {
	log.Info("Bootstrap balances from database.")

	for addr, _ := range s.accman.GetPairs() {
		// create inputs map for address
		s.inputs[addr] = make([]*types.UTxO, 0)

		// load user balance
		uoarr, err := s.outm.FindAllUTxO(addr)
		if err != nil {
			return errors.Wrap(err, "LoadBalances")
		}

		var balance uint64 = 0
		for _, uo := range uoarr {
			s.inputs[addr] = append(s.inputs[addr], uo)
			balance += uo.Amount
		}

		log.Infof("Load all outputs for address %s. Count: %d. Balance: %d", addr, len(uoarr), balance)

		// create counter of tx for user
		s.count[addr] = 0
	}

	return nil
}

// Start service work
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
				log.Errorf("generatorLoop: Error: %s", err.Error())

				if errors.Is(err, ErrUtxoSize) {
					log.Warn("Got wrong UTxO count. Reload balances.")
					err = s.loadBalances()

					// if no error try another
					if err == nil {
						continue
					}
				}

				if errors.Is(err, ErrEmptyAll) {
					log.Error("Break generator loop.")
					return
				}

				s.err <- err
				return
			}

			if s.fullStatFlag {
				end := time.Since(start)
				log.Infof("Create block in: %s", common.StatFmt(end))
				s.updateBlockStat(end)
			}

			log.Warnf("Block #%d generated.", num)
		}
	}
}

// initBlockStat prepare counters
func (s *LanSrv) initBlockStat() {
	s.blockStat["count"] = 0
	s.blockStat["max"] = 0
	s.blockStat["min"] = 1e12
	s.blockStat["sum"] = 0
}

// updateBlockStat update stats counters
func (s *LanSrv) updateBlockStat(end time.Duration) {
	s.lock.Lock()

	res := int64(end / time.Millisecond)

	if res > s.blockStat["max"] {
		s.blockStat["max"] = res
	}

	if res < s.blockStat["min"] {
		s.blockStat["min"] = res
	}

	s.blockStat["count"]++
	s.blockStat["sum"] += res

	s.lock.Unlock()
}

// genBlockWorker worker for creating one block and store it to the database
func (s *LanSrv) genBlockWorker() (uint64, error) {
	start := time.Now()

	// create tx batch and put generated transactions to the pool
	err := s.generateTxBatch()
	if err != nil {
		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("Generated transactions for block. Count: %d Time: %s", common.TxLimitPerBlock, common.StatFmt(end))
	}

	start = time.Now()

	// generate block with block miner
	block, err := s.miner.GenerateBlock()
	if err != nil {
		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genBlockWorker: Generate block in %s.", common.StatFmt(end))
	}

	start = time.Now()

	// validate, save block and update SQL
	err = s.miner.FinalizeBlock(block)
	if err != nil {
		log.Error("genBlockWorker: Error finalizing block.")
		s.restoreInputs()

		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genBlockWorker: Finalize block in %s.", common.StatFmt(end))
	}

	start = time.Now()

	// update local inputs map data
	s.UpdateBlockOutputs(block)

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("genBlockWorker: Update local inputs %s.", common.StatFmt(end))
	}

	return block.Num, nil
}

// UpdateBlockOutputs update in-memory outputs for users
func (s *LanSrv) UpdateBlockOutputs(block *prototype.Block) {
	// update local inputs
	var index uint32
	for _, tx := range block.Transactions {
		if tx.Type != common.NormalTxType && tx.Type != common.GenesisTxType {
			continue
		}

		from := tx.Inputs[0].Address
		index = 0
		for _, out := range tx.Outputs {
			uo := types.NewUTxO(tx.Hash, from, out.Address, out.Node, index, out.Amount, block.Num, int(tx.Type), 0)
			s.inputs[uo.To.Hex()] = append(s.inputs[uo.To.Hex()], uo)

			index++
		}
	}

	// reset inputs backup
	s.inputsTmp = make(map[string][]*types.UTxO)

	// reset already sent map after block creation
	s.alreadySent = make(map[string]int, common.AccountNum)
}

// Stop stops tx generator service
func (s *LanSrv) Stop() error {
	close(s.stop)  // close stop chan
	s.cancelFunc() // finish context

	s.showEndStats()

	log.Warn("Stop service.")
	return nil
}

// showEndStats write stats when stop service
func (s *LanSrv) showEndStats() {
	log.Printf("Blocks: %d Transactions: %d Utxo count: %d.", s.bc.GetCurrentBlockNum(), s.bc.GetCurrentBlockNum()*common.TxLimitPerBlock, s.lastUTxO)

	s.lock.Lock()

	var avg int64
	if s.blockStat["count"] > 0 {
		avg = s.blockStat["sum"] / s.blockStat["count"]
	} else {
		avg = 0
		s.blockStat["max"] = 0
		s.blockStat["min"] = 0
	}

	log.Printf("Blocks stat. Max: %d ms Min: %d ms Avg: %d ms.", s.blockStat["max"], s.blockStat["min"], avg)

	s.lock.Unlock()

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
			num := rmath.RandUint64(1, s.bc.GetBlockCount())

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
