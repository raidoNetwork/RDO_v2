package txgen

import (
	"encoding/binary"
	ssz "github.com/ferranbt/fastssz"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"math/rand"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	types "github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/keystore"
	"time"
)

var log = logrus.WithField("prefix", "tx generator")
var signSuffix []byte // byte slice to fill signature to the needed size

func NewService(ctx *cli.Context, db db.Database) *Service {
	signSuffix = make([]byte, 32)
	for i := 0; i < 32; i++ {
		signSuffix[i] = byte(rand.Int())
	}

	srv := &Service{
		db:        db,
		err:       make(chan error),
		stop:      make(chan struct{}),
		blkNum:    1,
		blkCount:  0,
		txNum:     0,
		prevHash:  make([]byte, 32),
		startTime: time.Now(),
		ctx:       ctx,
	}

	return srv
}

type Service struct {
	db        db.Database
	err       chan error
	stop      chan struct{}
	blkNum    uint64
	blkCount  int
	txNum     int
	prevHash  []byte
	startTime time.Time
	ctx       *cli.Context
	writeTest bool
	readTest  bool
}

func (s *Service) Start() {
	log.Warn("Start tx generator service.")

	s.writeTest = s.ctx.Bool(flags.DBWriteTest.Name)
	s.readTest = s.ctx.Bool(flags.DBReadTest.Name)

	if !s.writeTest && !s.readTest {
		return
	}

	// get blocks' count
	count, err := s.checkDB()
	if err != nil {
		log.Error("Error reading blocks' count from database.", err)
		return
	}

	// DB stats flag means that we need to show stat and nothing more.
	if s.ctx.Bool(flags.DBStats.Name) {
		log.Warnf("DB has #%d blocks.", count)
		return
	}

	s.blkNum = uint64(count + 1)

	// start write test
	if s.writeTest {
		go s.worker()
	}

	// start read test
	if s.readTest {
		go s.readWorker()
	}
}

func (s *Service) Stop() error {
	close(s.stop)

	log.Warn("Stop tx generator service")
	return nil
}

func (s *Service) Status() error {
	select {
	case err := <-s.err:
		return err
	default:
		return nil
	}
}

func (s *Service) checkDB() (count int, err error) {
	count, err = s.db.CountBlocks()
	if err != nil {
		return
	}

	log.Infof("Database has %d blocks.", count)
	return count, nil
}

func (s *Service) worker() {
	tx := s.genTx() // generate transaction

	// set db method for putting block in
	helper := s.db.WriteBlock // use keys like hash(block num + timestamp)

	// if write test flag is given change the put method func
	if s.writeTest {
		helper = s.db.WriteBlockWithNumKey // use keys like hash(blocknum + block suffix)
	}

	for {
		select {
		case <-s.stop:
			log.Infof("Storage stats. Blocks: %d. Transactions: %d. Time spent: %v.", s.blkCount-1, s.txNum, time.Since(s.startTime))
			log.Infof("Last block num: %d.", s.blkNum-1)
			return
		default:
			blk := s.genBlock(tx, false)

			err := helper(blk)
			if err != nil {
				log.Error("Error writing block. ", err)
				log.Errorf("Block num %d", s.blkNum)
				s.err <- err
				return
			}

			s.blkCount++
			log.Infof("Writed block #%d", s.blkNum-1)
		}
	}
}

func (s *Service) readWorker() {
	for {
		select {
		case <-s.stop:
			log.Infof("Readed blocks %d.", s.blkCount)
			return
		default:
			rand.Seed(time.Now().UnixNano())

			// generate key
			num := rand.Intn(int(s.blkNum)) + 1
			key := genKey(num)

			res, err := s.db.ReadBlock(key)
			if err != nil {
				log.Error("Error reading block.", err)
				return
			}

			if res == nil {
				continue
			}

			log.Infof("Block number #%d has been readed.", num)
			log.Infof("Success readings #%d", s.blkCount)
			s.blkCount++
		}
	}
}

func (s *Service) genBlock(tx *types.Transaction, randomTxCount bool) *types.Block {
	timestamp := time.Now().UnixNano()

	// if randomTxCount is true we put into the block random number of transactions in the range [1;100]
	// otherwise block fills with 100 transactions
	var txCount int
	if randomTxCount {
		rand.Seed(timestamp)
		txCount = rand.Intn(99) + 1 // +1 is needed to avoid txCount = 0 case
	} else {
		txCount = 100
	}

	transactions := make([]*types.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		transactions[i] = tx
	}

	s.txNum += txCount

	h := blockHash(s.prevHash, s.blkNum, timestamp) // hash

	block := &types.Block{
		Num:       s.blkNum,
		Slot:      s.blkNum,
		Version:   []byte{1, 0, 0},
		Hash:      h,
		Parent:    s.prevHash,
		Txroot:    h,
		Timestamp: uint64(timestamp),
		Size:      33,
		Proposer:  sign(s.prevHash, h),
		Approvers: []*types.Sign{
			sign(s.prevHash, h),
			sign(s.prevHash, h),
			sign(s.prevHash, h),
		},
		Slashers: []*types.Sign{
			sign(s.prevHash, h),
			sign(s.prevHash, h),
		},
		Transactions: transactions,
	}

	block.Size = uint32(block.SizeSSZ())

	s.blkNum++
	s.prevHash = h

	return block
}

func (s *Service) genTx() *types.Transaction {
	const (
		inputNum  = 6
		outputNum = 3
	)

	num := rand.Uint64()
	timestamp := time.Now().UnixNano()

	genInput := func() *types.TxInput {
		n := rand.Uint32()
		amount := rand.Uint64()
		timestamp := time.Now().UnixNano()

		res := &types.TxInput{
			Hash:    txHash(uint64(n), timestamp),
			Index:   n,
			Amount:  amount,
			Address: txHash(amount, timestamp),
			Sign:    sign([]byte{}, txHash(amount, timestamp)),
		}

		return res
	}

	genOutput := func() *types.TxOutput {
		amount := rand.Uint64()
		timestamp := time.Now().UnixNano()

		res := &types.TxOutput{
			Amount:  amount,
			Address: txHash(amount, timestamp),
		}

		return res
	}

	inputs := make([]*types.TxInput, inputNum)
	for i := 0; i < inputNum; i++ {
		inputs[i] = genInput()
	}

	outputs := make([]*types.TxOutput, outputNum)
	for i := 0; i < outputNum; i++ {
		outputs[i] = genOutput()
	}

	tx := &types.Transaction{
		Num:       num,
		Type:      1,
		Timestamp: uint64(timestamp),
		Hash:      txHash(num, timestamp),
		Fee:       rand.Uint64(),
		Size:      0,
		Data:      make([]byte, 1),
		Inputs:    inputs,
		Outputs:   outputs,
	}

	tx.Size = uint32(tx.SizeSSZ())

	return tx
}

func hashDomain(num uint64, timestamp int64) []byte {
	res := make([]byte, 0, 16)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, num)
	res = append(res, buf...)

	binary.BigEndian.PutUint64(buf, uint64(timestamp))
	res = append(res, buf...)

	return res
}

func txHash(num uint64, timestamp int64) []byte {
	res := hashDomain(num, timestamp)
	h := keystore.Keccak256(res)
	res = h[:]

	return res
}

func blockHash(prev []byte, num uint64, timestamp int64) []byte {
	res := hashDomain(num, timestamp)
	res = append(res, prev...)

	h := keystore.Keccak256(res)
	res = h[:]

	return res
}

func sign(prevHash []byte, hash []byte) *types.Sign {
	res := prevHash[:]

	if len(res) == 0 {
		res = make([]byte, 32) // null slice
	}

	res = append(res, hash...)
	res = append(res, signSuffix...) // need to fill sign to the size of 96 bytes

	sign := &types.Sign{
		Signature: res,
	}

	return sign
}

func genKey(num int) []byte {
	nbyte := make([]byte, 0)
	nbyte = ssz.MarshalUint64(nbyte, uint64(num))
	nbyte = append(nbyte, kv.BlockSuffix...)
	key := keystore.Keccak256(nbyte)

	return key
}
