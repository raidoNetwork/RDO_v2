package txpool

import (
	"context"
	"github.com/raidochain/proto/prototype"
	"github.com/raidochain/shared/common"
	"github.com/raidochain/shared/crypto"
	rmath "github.com/raidochain/shared/math"
	"github.com/raidochain/shared/types"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var num uint64 = 1
var globalLock = sync.RWMutex{}
var testTxStore = make([]*prototype.Transaction, 0)

// TestValidator implements TxValidator interface.
type TestValidator struct{}

func (tv *TestValidator) ValidateTransaction(tx *prototype.Transaction) error {
	return nil
}

func (tv *TestValidator) ValidateTransactionData(tx *prototype.Transaction) error {
	return nil
}

func getFee() uint64 {
	return rmath.RandUint64(1, 100)
}

func randHash() []byte {
	rand.Seed(time.Now().UnixNano())
	h := make([]byte, 32)
	rand.Read(h)

	return crypto.Keccak256(h)
}

func createTestTx() (*prototype.Transaction, error) {
	globalLock.Lock()
	txNum := num
	num++
	globalLock.Unlock()

	in, err := types.NewInput(
		randHash(),
		rand.Uint32(),
		types.NewOutput(crypto.Keccak256([]byte("from")), rmath.RandUint64(20000, 500000), nil),
		nil)

	if err != nil {
		return nil, err
	}

	opts := types.TxOptions{
		Num: txNum,
		Fee: getFee(),
		Inputs: []*prototype.TxInput{
			in,
		},
		Outputs: []*prototype.TxOutput{
			&prototype.TxOutput{
				Amount:  rmath.RandUint64(1, 202020),
				Address: make([]byte, 32),
			},
		},
	}

	tx, err := types.NewTx(opts, nil)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func createTestPool(size int, isFeeConst bool) (*TxPool, []*prototype.Transaction) {
	validator := &TestValidator{}
	pool := NewTxPool(validator)
	var constFeeVal uint64 = 13

	txArr := make([]*prototype.Transaction, size)

	// add tx to the pool
	for i := 0; i < size; i++ {
		tx, err := createTestTx()
		if err != nil {
			log.Fatalf("createTestTx error: %s", err.Error())
		}

		if isFeeConst {
			tx.Fee = constFeeVal
		}

		txArr[i] = tx

		err = pool.RegisterTx(tx)
		if err != nil {
			log.Fatalf("RegisterTx error: %s. For tx %s", err.Error(), common.Encode(tx.Hash))
		}
	}

	// sort tx arr
	sort.Slice(txArr, func(i, j int) bool {
		if txArr[i].Fee == txArr[j].Fee {
			return txArr[i].Timestamp < txArr[j].Timestamp
		}

		return txArr[i].Fee > txArr[j].Fee
	})

	return pool, txArr
}

func TestTxPoolSort(t *testing.T) {
	pool, txArr := createTestPool(100, false)

	pricedPool := pool.GetPricedQueue()

	for i, tx := range txArr {
		hash := common.Encode(tx.Hash)

		el := pricedPool[i]
		heapHash := common.Encode(el.GetTx().Hash)

		if hash != heapHash {
			t.Logf("TxArr: %s Fee: %d Time: %d", hash, tx.Fee, tx.Timestamp)
			t.Logf("Heap:  %s Fee: %d Time: %d", heapHash, el.GetTx().Fee, el.GetTx().Timestamp)
			t.Fatalf("Wrong sort order on index %d.", i)
		}
	}
}

func TestTxPoolSortWithSameFee(t *testing.T) {
	pool, txArr := createTestPool(100, true)
	pricedPool := pool.GetPricedQueue()

	for i, tx := range txArr {
		hash := common.Encode(tx.Hash)

		el := pricedPool[i]
		heapHash := common.Encode(el.GetTx().Hash)

		if hash != heapHash {
			t.Logf("TxArr: %s Fee: %d Time: %d", hash, tx.Fee, tx.Timestamp)
			t.Logf("Heap:  %s Fee: %d Time: %d", heapHash, el.GetTx().Fee, el.GetTx().Timestamp)
			t.Fatalf("Wrong sort order. Bad tx: %s.", heapHash)
		}
	}
}

func TestTxPoolRemoveTx(t *testing.T) {
	pool, txArr := createTestPool(10, false)
	_ = pool.GetPricedQueue() // sort pool

	i := len(txArr)
	for i > 0 {
		index := rand.Intn(i) // element index
		el := txArr[index]
		err := pool.ReserveTransactions([]*prototype.Transaction{
			el,
		})

		if err != nil {
			t.Logf("Index %d.", index)
			t.Fatal(err)
		}

		// check that el is removed successfully
		findex, err := pool.pricedPool.FindByTx(el)
		if err == nil {
			t.Fatal("No error with undefined tx.")
		}

		if findex > -1 {
			t.Fatal("Tx wasn't removed from TxPool")
		}

		txArr = append(txArr[:index], txArr[index+1:]...)

		i = len(txArr)
		t.Logf("New array len is %d. Real len %d", i, pool.pricedPool.Len())
	}
}

func TestDoubleTx(t *testing.T) {
	pool, txArr := createTestPool(1, false)

	err := pool.RegisterTx(txArr[0])
	if err == nil {
		t.Fatal("Add one tx twice!")
	}

	t.Log(err)
}

func TestDoubleSpend(t *testing.T) {
	size := 3
	pool, txArr := createTestPool(size, false)

	tx, err := createTestTx()
	if err != nil {
		t.Fatal(err)
	}

	// create double spend
	tx.Inputs[0] = txArr[rand.Intn(size)].Inputs[0]

	err = pool.RegisterTx(tx)
	if err == nil {
		t.Fatal("Spent one input twice.")
	}

	t.Log(err)
}

func BenchmarkSortData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool, _ := createTestPool(200, false)

		// sort by price
		_ = pool.GetPricedQueue()
	}
}

func TestConcurentRegister(t *testing.T) {
	pool, _ := createTestPool(0, false)

	stopTx := make(chan struct{})
	stop := make(chan struct{})
	err := make(chan error)

	wg := sync.WaitGroup{}

	// generate transactions
	go createTxWorker(pool, stopTx, err)
	go createTxWorker(pool, stopTx, err)

	// error proccess
	go func(errc chan error) {
		for err := range errc {
			t.Logf("Got worker error: %v.", err)
		}
	}(err)

	// lock for two second and generate some transactions
	<-time.After(2 * time.Second)

	wg.Add(1)
	go createBlockWorker(pool, stop, err, &wg)

	// stop tx generation
	close(stopTx)

	time.Sleep(6 * time.Second)

	// stop block generator
	close(stop)

	// wait for block generator
	wg.Wait()

	// wait goroutines
	for _, tx := range testTxStore {
		hash := common.Encode(tx.Hash)
		res := pool.checkTxOut(tx)

		if !res {
			t.Fatalf("TX %s is in pool yet.", hash)
			return
		}
	}
}

func createTxWorker(pool *TxPool, stop chan struct{}, errc chan error) {
	for {
		select {
		case <-stop:
			return
		default:
			tx, err := createTestTx()
			if err != nil {
				errc <- err
				return
			}

			err = pool.RegisterTx(tx)
			if err != nil {
				errc <- err
				return
			}
		}
	}
}

func createBlockWorker(pool *TxPool, stop chan struct{}, errc chan error, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-stop:
			return
		default:
			txBatch := getTxBatch(pool)

			if len(txBatch) == 0 {
				return
			}

			err := pool.ReserveTransactions(txBatch)
			if err != nil {
				errc <- err
				return
			}

			// save given batch to the test store
			testTxStore = append(testTxStore, txBatch...)
		}
	}
}

func getTxBatch(pool *TxPool) []*prototype.Transaction {
	txList := pool.GetPricedQueue()
	txBatch := make([]*prototype.Transaction, 0)

	txListSize := len(txList)

	totalSize := 0         // current size of block in bytes
	BlockSize := 10 * 1024 // 10kb

	var size int
	for i := 0; i < txListSize; i++ {
		size = txList[i].Size()
		totalSize += size

		if totalSize <= BlockSize {
			txBatch = append(txBatch, txList[i].GetTx())

			// we fill block successfully
			if totalSize == BlockSize {
				break
			}
		} else {
			// tx is too big try for look up another one
			totalSize -= size
		}
	}

	return txBatch
}

func TestFeed(t *testing.T) {
	pool, _ := createTestPool(0, false)
	ctx, finish := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	ch := make(chan *prototype.Transaction)

	var counter int32 = 0

	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func(ctx context.Context, n int, wg *sync.WaitGroup, count *int32, fin context.CancelFunc) {
			defer func() {
				wg.Done()
				t.Logf("Stop goroutine %d.", n)
			}()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					tx, err := createTestTx()
					if err != nil {
						t.Logf("Got error %s.", err)
						return
					}

					atomic.AddInt32(count, 1)

					t.Log(strings.Repeat(" |", n) + "▉")

					err = pool.SendTx(tx)
					if err != nil {
						t.Logf("Got error: %s", err)
						fin()
					}
				}
			}
		}(ctx, i, &wg, &counter, finish)
	}

	<-time.After(3 * time.Second)

	finish()

	wg.Wait()
	close(ch)

	txArr := pool.GetPricedQueue()

	t.Logf("Tx in pool %d. Count len: %d", len(txArr), counter)
}
