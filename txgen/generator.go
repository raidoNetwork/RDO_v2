package txgen

import (
	"context"
	"crypto/ecdsa"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	rmath "github.com/raidoNetwork/RDO_v2/shared/math"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	outputsLimit = 5

	generatorInterval        = 500 * time.Millisecond
	txPerTickCount           = 25
	accountsStep             = 5
	startAmount       uint64 = 1e12 //10000000000000 // 1 * 10e12
)

var (
	ErrEmptyAll    = errors.New("All balances are empty.")
	ErrAccountsNum = errors.New("Too small accounts number.")
	ErrNullChange  = errors.New("Null change.")
)

var log = logrus.WithField("prefix", "TxGenerator")

func NewGenerator(cliCtx *cli.Context) (*TxGenerator, error) {
	accman, err := prepareAccounts(cliCtx)
	if err != nil {
		log.Errorf("Error generating accounts. %s", err)
		return nil, err
	}

	host := cliCtx.String(flags.RPCHost.Name)
	port := cliCtx.String(flags.RPCPort.Name)
	datadir := cliCtx.String(cmd.DataDirFlag.Name)

	api, err := NewClient(host + ":" + port)
	if err != nil {
		log.Errorf("Error creating API client: %s", err)
		return nil, err
	}

	// setup default config
	params.UseMainnetConfig()

	// get custom config
	cfg := params.RaidoConfig()

	ctx, cancel := context.WithCancel(context.Background())

	tg := TxGenerator{
		nullBalance:   0,
		accman:        accman,
		mu:            sync.RWMutex{},
		ctx:           ctx,
		cancel:        cancel,
		cli:           cliCtx,
		stop:          make(chan struct{}),
		api:           api,
		senderIndex:   0,
		receiverIndex: 0,
		dataDir:       datadir,
		cfg:           cfg,
		txCounter:     0,
	}

	return &tg, nil
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
		log.Errorf("Error loading accounts: %s", err)
		return nil, err
	}

	// if keys are not stored create new key pairs and store them
	if errors.Is(err, types.ErrEmptyKeyDir) {
		log.Warn("Create new accounts.")

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

type TxGenerator struct {
	nullBalance int

	accman *types.AccountManager // key pair storage

	mu   sync.RWMutex
	stop chan struct{} // chan for main thread lock

	cancel context.CancelFunc
	ctx    context.Context
	cli    *cli.Context

	api *Client // RDO API client

	senderIndex   int // current transaction sender index
	receiverIndex int // current transaction receiver index

	dataDir string

	cfg         *params.RDOBlockChainConfig
	stakeAmount uint64

	txCounter int
}

// Start generator service
func (tg *TxGenerator) Start() {
	log.Warn("Start tx generator.")

	tg.mu.Lock()
	stop := tg.stop
	tg.mu.Unlock()

	tg.stakeAmount = tg.cfg.StakeSlotUnit * tg.cfg.RoiPerRdo

	go tg.api.Start()

	// start main loop
	go tg.loop()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")

		go tg.Stop()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic")
			}
		}
		panic("Panic closing the tx generator")
	}()

	<-stop
}

// Stop generator service
func (tg *TxGenerator) Stop() {
	tg.cancel()
	close(tg.stop)

	tg.api.Stop()
}

// loop generator main loop
func (tg *TxGenerator) loop() {
	ticker := time.NewTicker(generatorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tg.ctx.Done():
			return
		case <-ticker.C:
			// prepare and send tx batch to the pool
			// and lock till block will be generated
			err := tg.sendTxBatch()
			if err != nil {
				log.Errorf("Break generator loop. Error: %s", err)

				if strings.Contains(err.Error(), "rpc") {
					go tg.restartGenerator()
				} else {
					tg.Stop() // shutdown
				}

				return
			}
		}
	}
}

// createTx create tx
func (tg *TxGenerator) createTx() (*prototype.Transaction, error) {
	if tg.nullBalance >= common.AccountNum {
		log.Errorf("Null balances count: %d. Transactions: %d. Current sender: %d", tg.nullBalance, tg.txCounter, tg.senderIndex)
		return nil, ErrEmptyAll
	}

	startMain := time.Now()

	// choose transaction sender
	from, err := tg.chooseSender()
	if err != nil {
		return nil, err
	}

	var txType uint32 = common.NormalTxType
	var userInputs []*types.UTxO
	var start time.Time
	var end time.Duration

	// every number in sequence [1, 100] has a probability to be chosen in 1%.
	percent := rand.Intn(100) + 1
	if percent == 1 {
		start = time.Now()

		userInputs, err = tg.api.FindStakeDeposits(from)
		if err != nil {
			return nil, err
		}

		end = time.Since(start)
		log.Infof("Address: %s Deposits count: %d Got in: %s.", from, len(userInputs), common.StatFmt(end))

		// if user made stakes earlier create Unstake transaction
		// otherwise try to stake some money
		if len(userInputs) > 0 {
			txType = common.UnstakeTxType
		} else {
			txType = common.StakeTxType
		}
	}

	start = time.Now()

	if txType != common.UnstakeTxType {
		userInputs, err = tg.api.FindAllUTxO(from)
		if err != nil {
			return nil, err
		}
	}

	// create inputs for unstake tx
	userInputsLen := len(userInputs)
	inputsArr := make([]*prototype.TxInput, 0, userInputsLen)

	// count balance
	var balance uint64
	for _, uo := range userInputs {
		balance += uo.Amount
		inputsArr = append(inputsArr, uo.ToInput())
	}

	end = time.Since(start)
	log.Infof("Address: %s UTxO count: %d SenderIndex %d Get balance in: %s.", from, userInputsLen, tg.senderIndex-1, common.StatFmt(end))

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		log.Warnf("Address: %s has balance 0. TxType %d", from, txType)
		tg.nullBalance++

		return nil, errors.New("Zero balance on the wallet.")
	}

	// if user has balance equal to the minimum fee find another user
	if balance <= tg.cfg.MinimalFee {
		log.Warnf("Account %s has low balance equal or smaller than minimum fee.", from)
		return tg.createTx()
	}

	// check address having enough balance for staking
	// if not create regular transaction
	if txType == common.StakeTxType && balance <= tg.stakeAmount {
		log.Warnf("Cann't create stake transaction for %s. Balance too low.", from)
		txType = common.NormalTxType
	}

	// get account nonce
	var nonce uint64 = 1 // TODO add API method GetNonce

	// get transaction fee
	fee := tg.getFee()

	// create transaction base data
	opts := types.TxOptions{
		Fee:    fee,
		Num:    nonce + 1,
		Data:   []byte{},
		Type:   txType,
		Inputs: inputsArr,
	}

	// generate amount to spend for transaction
	targetAmount := tg.stakeAmount
	if opts.Type == common.NormalTxType {
		targetAmount = rmath.RandUint64(1, balance)
	}

	log.Infof("Address: %s Balance: %d. Trying to spend: %d.", from, balance, targetAmount)

	var change uint64

	if txType != common.UnstakeTxType {
		// count address change and update opts outputs with change one
		change, err = tg.findChange(from, balance, targetAmount, &opts)
		if err != nil {
			return nil, err
		}
	} else {
		change = 0
	}

	// get user private key for signing inputs
	usrKey := tg.accman.GetKey(from)

	// create and sign transaction
	tx, err := tg.createTxBody(opts, from, change, targetAmount, usrKey, balance)
	if err != nil {
		return nil, err
	}

	// log message
	entry := "Generated "
	if opts.Type == common.StakeTxType {
		entry += "stake"
	} else if opts.Type == common.UnstakeTxType {
		entry += "unstake"
	} else {
		entry += "normal"
	}

	end = time.Since(startMain)
	log.Warnf(entry+" tx %s in %s.", common.BytesToHash(tx.Hash), common.StatFmt(end))

	tg.txCounter++

	return tx, nil
}

// findChange count change for transaction and creates change output.
func (tg *TxGenerator) findChange(from string, balance uint64, targetAmount uint64, opts *types.TxOptions) (uint64, error) {
	change := balance - targetAmount // user change

	if change == 0 {
		return 0, ErrNullChange
	}

	// create change output
	addr := common.HexToAddress(from)
	out := types.NewOutput(addr.Bytes(), change, nil)
	opts.Outputs = append(opts.Outputs, out)

	return change, nil
}

// getStakeOutput creates stake output for given address
func (tg *TxGenerator) getStakeOutput(from string) *prototype.TxOutput {
	return types.NewOutput(common.HexToAddress(from).Bytes(),
		tg.stakeAmount,
		common.HexToAddress(common.BlackHoleAddress).Bytes())
}

// getStakeOutput creates stake output for given address
func (tg *TxGenerator) getUnstakeOutput(from string) *prototype.TxOutput {
	return types.NewOutput(common.HexToAddress(from).Bytes(),
		tg.stakeAmount,
		nil)
}

// createTxBody generate transaction outputs and create transaction struct. Also sign transaction with address private key.
func (tg *TxGenerator) createTxBody(opts types.TxOptions,
	from string,
	change, targetAmount uint64,
	usrKey *ecdsa.PrivateKey,
	balance uint64) (*prototype.Transaction, error) {

	// generate outputs and add it to the tx
	if opts.Type == common.NormalTxType {
		opts.Outputs = append(opts.Outputs, tg.generateTxOutputs(targetAmount)...)
	} else if opts.Type == common.StakeTxType {
		opts.Outputs = append(opts.Outputs, tg.getStakeOutput(from))
	} else {
		opts.Outputs = append(opts.Outputs, tg.getUnstakeOutput(from))
	}

	tx, err := types.NewTx(opts, usrKey)
	if err != nil {
		return nil, err
	}

	realFee := tx.GetRealFee()

	errFeeTooBig := errors.Errorf("tx can't pay fee. Balance: %d. Value: %d. Fee: %d. Change: %d.", balance, targetAmount, realFee, change)

	if opts.Type == common.UnstakeTxType {
		tx.Outputs[0].Amount -= realFee
	} else {
		if change > realFee {
			change -= realFee
			tx.Outputs[0].Amount = change
		} else {
			return nil, errFeeTooBig
		}
	}

	// We change tx outputs so we need to update tx hash.
	// That will make tx hash correct according to changes.
	hash, err := hasher.TxHash(tx)
	if err != nil {
		return nil, err
	}

	tx.Hash = hash[:]

	// Tx sign depends on tx hash.
	// When hash has been changed it is need to update signature.
	err = types.SignTx(tx, usrKey)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// chooseSender choose tx sender and receiver start index
func (tg *TxGenerator) chooseSender() (string, error) {
	lst := tg.accman.GetPairsList()

	if len(lst) != common.AccountNum {
		return "", ErrAccountsNum
	}

	sendPair := lst[tg.senderIndex]
	from := sendPair.Address

	if tg.senderIndex%accountsStep == 0 {
		tg.receiverIndex = tg.senderIndex - accountsStep

		if tg.receiverIndex < 0 {
			tg.receiverIndex += common.AccountNum
		}
	}

	tg.senderIndex++
	if tg.senderIndex == common.AccountNum {
		tg.senderIndex = 0
		tg.nullBalance = 0 // for fair balances check
	}

	return from.Hex(), nil
}

// generateTxOutputs create tx outputs with given total amount
func (tg *TxGenerator) generateTxOutputs(targetAmount uint64) []*prototype.TxOutput {
	outAmount := targetAmount
	lst := tg.accman.GetPairsList()
	outputCount := rand.Intn(outputsLimit) + 1 // outputs size limit

	var out *prototype.TxOutput
	res := make([]*prototype.TxOutput, 0)

	receiverIndex := tg.receiverIndex

	for i := 0; i < outputCount; i++ {
		// all money are spent
		if outAmount == 0 {
			break
		}

		pair := lst[receiverIndex] // get account receiver
		to := pair.Address

		// get random amount
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

		outAmount -= amount

		receiverIndex++
		if receiverIndex == common.AccountNum {
			receiverIndex = 0
		}
	}

	return res
}

// sendTxBatch create certain number of transactions and send it to the blockchain.
func (tg *TxGenerator) sendTxBatch() error {
	txCounter := 0
	start := time.Now()

	for i := 0; i < txPerTickCount; i++ {
		tx, err := tg.createTx()
		if err != nil {
			log.Errorf("Error creating tx: %s", err)

			// error with senders: something wrong with balances or rpc error
			if errors.Is(err, ErrEmptyAll) || errors.Is(err, ErrAccountsNum) || strings.Contains(err.Error(), "rpc error") {
				return err
			}

			continue
		}

		err = tg.sendTx(tx)
		if err != nil {
			log.Errorf("Error sending tx: %s", err)

			if strings.Contains(err.Error(), "rpc error") {
				return err
			}

			continue
		}

		txCounter++
		log.Infof("Tx %s send to the Tx pool.", common.BytesToHash(tx.Hash).Hex())
	}

	end := time.Since(start)
	log.Warnf("Create tx batch with size %d in time %s.", txCounter, common.StatFmt(end))

	return nil
}

// sendTx send tx to the blockchain
func (tg *TxGenerator) sendTx(tx *prototype.Transaction) error {
	return tg.api.SendTx(tx)
}

// restartGenerator restarts generator loop after timeout
func (tg *TxGenerator) restartGenerator() {
	<-time.After(TimeoutgRPC + 1*time.Second)

	log.Info("Restart generator loop.")

	go tg.loop()
}

// getFee generates random fee in given amount
func (tg *TxGenerator) getFee() uint64 {
	maxFee := tg.cfg.MinimalFee + 100
	return rmath.RandUint64(tg.cfg.MinimalFee, maxFee)
}
