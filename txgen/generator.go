package txgen

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	rmath "github.com/raidoNetwork/RDO_v2/shared/math"
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
	maxFee = 100 // max fee value
	minFee = 1   // min fee value

	outputsLimit = 4

	generatorInterval   = 7 * time.Second
	testAccountsNum     = 50
	testTxLimitPerBlock = 5
	accountsStep        = 5
)

var (
	ErrEmptyAll = errors.New("All balances are empty.")
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

	api, err := NewClient(host + ":" + port)
	if err != nil {
		log.Errorf("Error creating API client: %s", err)
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	tg := TxGenerator{
		nullBalance: 0,
		accman:      accman,
		mu:          sync.RWMutex{},
		ctx:         ctx,
		cancel:      cancel,
		cli:         cliCtx,
		count:       map[string]uint64{},
		stop:        make(chan struct{}),
		api:         api,
		senderIndex: 0,
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
		err = accman.CreatePairs(testAccountsNum)
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
	count  map[string]uint64     // accounts tx counter

	mu   sync.RWMutex
	stop chan struct{} // chan for main thread lock

	cancel context.CancelFunc
	ctx    context.Context
	cli    *cli.Context

	api *Client // RDO API client

	senderIndex int // current transaction sender index
}

// Start generator service
func (tg *TxGenerator) Start() {
	log.Warn("Start tx generator.")

	tg.mu.Lock()
	stop := tg.stop
	tg.mu.Unlock()

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

	counter := 0
	step := testAccountsNum / testTxLimitPerBlock
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
				}

				return
			}

			if counter%step == 0 {
				tg.nullBalance = 0
			}

			counter++
		}
	}
}

// createTx create tx using in-memory inputs
func (tg *TxGenerator) createTx() (*prototype.Transaction, error) {
	log.Infof("createTx: Attempt to create tx.")

	if tg.nullBalance >= testAccountsNum {
		return nil, ErrEmptyAll
	}

	from, receiverIndex := tg.chooseSender()

	start := time.Now()
	userInputs, err := tg.api.FindAllUTxO(from)
	if err != nil {
		return nil, err
	}

	userInputsLen := len(userInputs)
	inputsArr := make([]*prototype.TxInput, 0, userInputsLen)

	// get user private key for signing inputs
	usrKey := tg.accman.GetKey(from)

	// count balance
	var balance uint64
	for _, uo := range userInputs {
		balance += uo.Amount

		inputsArr = append(inputsArr, uo.ToInput(usrKey))
	}

	log.Infof("Adress %s UTxO count: %d Balance: %d", from, userInputsLen, balance)

	end := time.Since(start)
	log.Infof("createTx: Get address balance in %s.", common.StatFmt(end))

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		log.Warnf("Account %s has balance %d.", from, balance)
		tg.nullBalance++
		return tg.createTx()
	}

	// if user has balance equal to the minimum fee find another user
	if balance == minFee {
		log.Warnf("Account %s has balanc equal to the minimum fee.", from)
		return tg.createTx()
	}

	fee := getFee() // get price for 1 byte of tx
	opts := types.TxOptions{
		Fee:    fee,
		Num:    tg.count[from],
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
	opts.Outputs = append(opts.Outputs, tg.generateTxOutputs(targetAmount, receiverIndex)...)

	end = time.Since(start)
	log.Infof("createTx: Generate outputs. Count: %d. Time: %s.", len(opts.Outputs), common.StatFmt(end))

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

	// Tx sign depends on tx hash.
	// When hash has been changed it is need to update signature.
	err = types.SignTx(tx, usrKey)
	if err != nil {
		return nil, err
	}

	log.Warnf("Generated tx %s", hash.Hex())

	return tx, nil
}

// chooseSender choose tx sender and receiver start index
func (tg *TxGenerator) chooseSender() (string, int) {
	lst := tg.accman.GetPairsList()
	sendPair := lst[tg.senderIndex]
	from := sendPair.Address

	receiverIndex := tg.senderIndex - accountsStep
	if receiverIndex < 0 {
		receiverIndex += testAccountsNum
	}

	tg.senderIndex++
	if tg.senderIndex == testAccountsNum {
		tg.senderIndex = 0
	}

	return from.Hex(), receiverIndex
}

// generateTxOutputs create tx outputs with given total amount
func (tg *TxGenerator) generateTxOutputs(targetAmount uint64, receiverIndex int) []*prototype.TxOutput {
	outAmount := targetAmount
	lst := tg.accman.GetPairsList()
	outputCount := rand.Intn(outputsLimit) + 1 // outputs size limit

	var out *prototype.TxOutput
	res := make([]*prototype.TxOutput, 0)

	for i := 0; i < outputCount; i++ {
		// all money are spent
		if outAmount == 0 {
			break
		}

		pair := lst[receiverIndex] // get account receiver
		to := pair.Address

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

		outAmount -= amount

		receiverIndex++
		if receiverIndex == testAccountsNum {
			receiverIndex = 0
		}
	}

	return res
}

// sendTxBatch create certain number of transactions and send it to the blockchain.
func (tg *TxGenerator) sendTxBatch() error {
	for i := 0; i < testTxLimitPerBlock; i++ {
		tx, err := tg.createTx()
		if err != nil {
			log.Errorf("Error creating tx: %s", err)

			// error with senders: something wrong with balances or rpc error
			if errors.Is(err, ErrEmptyAll) || strings.Contains(err.Error(), "rpc error") {
				return err
			}

			i-- // add an attempt to cover current failure
			continue
		}

		err = tg.sendTx(tx)
		if err != nil {
			log.Errorf("Error sending tx: %s", err)

			if strings.Contains(err.Error(), "rpc error") {
				return err
			}

			i-- // add an attempt to cover current failure
			continue
		}

		log.Infof("Tx %s send to the Tx pool.", common.BytesToHash(tx.Hash).Hex())
	}

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
func getFee() uint64 {
	return rmath.RandUint64(minFee, maxFee)
}
