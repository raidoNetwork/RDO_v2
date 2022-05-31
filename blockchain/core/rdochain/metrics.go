package rdochain

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
)

var (
	netRewardAmount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rdo_network_rewards",
		Help: "Minted roi with rewards",
	})
	netFeeAmount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rdo_network_fees",
		Help: "Burned roi with fees",
	})
	netBalances = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rdo_network_balances",
		Help: "Sum of user balances",
	})
	netGenesisAmount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "rdo_network_genesis_balance",
		Help: "Amount of roi stored in the Genesis",
	})
	headBlockNum = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "head_block_num",
		Help: "Head block number",
	})
	blockSavingTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "store_block_main_db_sum_time",
		Help: "Block storing width additional info duration",
		Buckets: common.MillisecondsBuckets,
	})
	blockHeadSavingTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "store_block_head_time",
		Help: "Storing block head pointer duration",
		Buckets: common.KVMillisecondsBuckets,
	})
	supplySavingTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "store_supply_data_time",
		Help: "Storing supply data duration",
		Buckets: common.KVMillisecondsBuckets,
	})
	inputsSavingTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "store_input_time",
		Help: "Storing transaction inputs time",
		Buckets: common.KVMillisecondsBuckets,
	})
	updateTxDataTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "update_tx_data_time",
		Help: "Update transaction data duration",
		Buckets: common.KVMillisecondsBuckets,
	})
	storeTransactionsTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "store_all_tx_time",
		Help: "All transactions update data time",
		Buckets: common.MillisecondsBuckets,
	})
	localDbSyncTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "local_db_sync_time",
		Help: "KV with SQL sync time",
		Buckets: common.MillisecondsBuckets,
	})
	localDbSyncBlockTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "local_db_sync_block_time",
		Help: "KV with SQL block sync time",
		Buckets: common.MillisecondsBuckets,
	})
)

func updateBalanceMetrics(rewards, fees, balances uint64) {
	netRewardAmount.Set(float64(rewards))
	netFeeAmount.Set(float64(fees))
	netBalances.Set(float64(balances))
}

func setStartupMetrics(block *prototype.Block, headBlockNumber uint64) {
	var balance uint64
	for _, tx := range block.Transactions {
		for _, out := range tx.Outputs {
			balance += out.Amount
		}
	}

	netGenesisAmount.Set(float64(balance))
	headBlockNum.Set(float64(headBlockNumber))
}