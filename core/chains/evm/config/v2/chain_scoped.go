package v2

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/multierr"

	ocr "github.com/smartcontractkit/libocr/offchainreporting"
	ocrtypes "github.com/smartcontractkit/libocr/offchainreporting/types"

	"github.com/smartcontractkit/chainlink/v2/core/assets"
	"github.com/smartcontractkit/chainlink/v2/core/chains/evm/config"
	gencfg "github.com/smartcontractkit/chainlink/v2/core/config"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
)

func NewTOMLChainScopedConfig(genCfg gencfg.AppConfig, chain *EVMConfig, lggr logger.Logger) *ChainScoped {
	return &ChainScoped{AppConfig: genCfg, cfg: chain, lggr: lggr}
}

// ChainScoped implements config.ChainScopedConfig with a gencfg.BasicConfig and EVMConfig.
type ChainScoped struct {
	gencfg.AppConfig
	lggr logger.Logger

	cfg *EVMConfig
}

func (c *ChainScoped) ChainID() *big.Int {
	return c.cfg.ChainID.ToInt()
}

func (c *ChainScoped) ChainType() gencfg.ChainType {
	if c.cfg.ChainType == nil {
		return ""
	}
	return gencfg.ChainType(*c.cfg.ChainType)
}

func (c *ChainScoped) Validate() (err error) {
	// Most per-chain validation is done on startup, but this combines globals as well.
	lc := ocrtypes.LocalConfig{
		BlockchainTimeout:                      c.OCR().BlockchainTimeout(),
		ContractConfigConfirmations:            c.OCRContractConfirmations(),
		ContractConfigTrackerPollInterval:      c.OCR().ContractPollInterval(),
		ContractConfigTrackerSubscribeInterval: c.OCR().ContractSubscribeInterval(),
		ContractTransmitterTransmitTimeout:     c.OCRContractTransmitterTransmitTimeout(),
		DatabaseTimeout:                        c.OCRDatabaseTimeout(),
		DataSourceTimeout:                      c.OCR().ObservationTimeout(),
		DataSourceGracePeriod:                  c.OCRObservationGracePeriod(),
	}
	if ocrerr := ocr.SanityCheckLocalConfig(lc); ocrerr != nil {
		err = multierr.Append(err, ocrerr)
	}
	return
}

type evmConfig struct {
	c *EVMConfig
}

func (e *evmConfig) BalanceMonitor() config.BalanceMonitor {
	return &balanceMonitorConfig{c: e.c.BalanceMonitor}
}

func (e *evmConfig) Transactions() config.Transactions {
	return &transactionsConfig{c: e.c.Transactions}
}

func (c *ChainScoped) EVM() config.EVM {
	return &evmConfig{c: c.cfg}
}

func (c *ChainScoped) AutoCreateKey() bool {
	return *c.cfg.AutoCreateKey
}

func (c *ChainScoped) BlockBackfillDepth() uint64 {
	return uint64(*c.cfg.BlockBackfillDepth)
}

func (c *ChainScoped) BlockBackfillSkip() bool {
	return *c.cfg.BlockBackfillSkip
}

type balanceMonitorConfig struct {
	c BalanceMonitor
}

func (b *balanceMonitorConfig) Enabled() bool {
	return *b.c.Enabled
}

func (c *ChainScoped) BlockEmissionIdleWarningThreshold() time.Duration {
	return c.NodeNoNewHeadsThreshold()
}

func (c *ChainScoped) BlockHistoryEstimatorBatchSize() (size uint32) {
	return *c.cfg.GasEstimator.BlockHistory.BatchSize
}

func (c *ChainScoped) BlockHistoryEstimatorBlockDelay() uint16 {
	return *c.cfg.RPCBlockQueryDelay
}

func (c *ChainScoped) BlockHistoryEstimatorBlockHistorySize() uint16 {
	return *c.cfg.GasEstimator.BlockHistory.BlockHistorySize
}

func (c *ChainScoped) BlockHistoryEstimatorCheckInclusionBlocks() uint16 {
	return *c.cfg.GasEstimator.BlockHistory.CheckInclusionBlocks
}

func (c *ChainScoped) BlockHistoryEstimatorCheckInclusionPercentile() uint16 {
	return *c.cfg.GasEstimator.BlockHistory.CheckInclusionPercentile
}

func (c *ChainScoped) BlockHistoryEstimatorEIP1559FeeCapBufferBlocks() uint16 {
	if c.cfg.GasEstimator.BlockHistory.EIP1559FeeCapBufferBlocks == nil {
		return uint16(c.EvmGasBumpThreshold()) + 1
	}
	return *c.cfg.GasEstimator.BlockHistory.EIP1559FeeCapBufferBlocks
}

func (c *ChainScoped) BlockHistoryEstimatorTransactionPercentile() uint16 {
	return *c.cfg.GasEstimator.BlockHistory.TransactionPercentile
}

func (c *ChainScoped) EvmEIP1559DynamicFees() bool {
	return *c.cfg.GasEstimator.EIP1559DynamicFees
}

type transactionsConfig struct {
	c Transactions
}

func (t *transactionsConfig) ForwardersEnabled() bool {
	return *t.c.ForwardersEnabled
}

func (t *transactionsConfig) ReaperInterval() time.Duration {
	return t.c.ReaperInterval.Duration()
}

func (t *transactionsConfig) ReaperThreshold() time.Duration {
	return t.c.ReaperThreshold.Duration()
}

func (t *transactionsConfig) ResendAfterThreshold() time.Duration {
	return t.c.ResendAfterThreshold.Duration()
}

func (t *transactionsConfig) MaxInFlight() uint32 {
	return *t.c.MaxInFlight
}

func (t *transactionsConfig) MaxQueued() uint64 {
	return uint64(*t.c.MaxQueued)
}

func (c *ChainScoped) EvmFinalityDepth() uint32 {
	return *c.cfg.FinalityDepth
}

func (c *ChainScoped) EvmGasBumpPercent() uint16 {
	return *c.cfg.GasEstimator.BumpPercent
}

func (c *ChainScoped) EvmGasBumpThreshold() uint64 {
	return uint64(*c.cfg.GasEstimator.BumpThreshold)
}

func (c *ChainScoped) EvmGasBumpTxDepth() uint32 {
	if c.cfg.GasEstimator.BumpTxDepth != nil {
		return *c.cfg.GasEstimator.BumpTxDepth
	}
	return *c.cfg.Transactions.MaxInFlight
}

func (c *ChainScoped) EvmGasBumpWei() *assets.Wei {
	return c.cfg.GasEstimator.BumpMin
}

func (c *ChainScoped) EvmGasFeeCapDefault() *assets.Wei {
	return c.cfg.GasEstimator.FeeCapDefault
}

func (c *ChainScoped) EvmGasLimitDefault() uint32 {
	return *c.cfg.GasEstimator.LimitDefault
}

func (c *ChainScoped) EvmGasLimitMax() uint32 {
	return *c.cfg.GasEstimator.LimitMax
}

func (c *ChainScoped) EvmGasLimitMultiplier() float32 {
	f, _ := c.cfg.GasEstimator.LimitMultiplier.BigFloat().Float32()
	return f
}

func (c *ChainScoped) EvmGasLimitTransfer() uint32 {
	return *c.cfg.GasEstimator.LimitTransfer
}

func (c *ChainScoped) EvmGasLimitOCRJobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.OCR
}

func (c *ChainScoped) EvmGasLimitOCR2JobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.OCR2
}

func (c *ChainScoped) EvmGasLimitDRJobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.DR
}

func (c *ChainScoped) EvmGasLimitVRFJobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.VRF
}

func (c *ChainScoped) EvmGasLimitFMJobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.FM
}

func (c *ChainScoped) EvmGasLimitKeeperJobType() *uint32 {
	return c.cfg.GasEstimator.LimitJobType.Keeper
}

func (c *ChainScoped) EvmGasPriceDefault() *assets.Wei {
	return c.cfg.GasEstimator.PriceDefault
}

func (c *ChainScoped) EvmMinGasPriceWei() *assets.Wei {
	return c.cfg.GasEstimator.PriceMin
}

func (c *ChainScoped) EvmMaxGasPriceWei() *assets.Wei {
	return c.cfg.GasEstimator.PriceMax
}

func (c *ChainScoped) EvmGasTipCapDefault() *assets.Wei {
	return c.cfg.GasEstimator.TipCapDefault
}

func (c *ChainScoped) EvmGasTipCapMinimum() *assets.Wei {
	return c.cfg.GasEstimator.TipCapMin
}

func (c *ChainScoped) EvmHeadTrackerHistoryDepth() uint32 {
	return *c.cfg.HeadTracker.HistoryDepth
}

func (c *ChainScoped) EvmHeadTrackerMaxBufferSize() uint32 {
	return *c.cfg.HeadTracker.MaxBufferSize
}

func (c *ChainScoped) EvmHeadTrackerSamplingInterval() time.Duration {
	return c.cfg.HeadTracker.SamplingInterval.Duration()
}

func (c *ChainScoped) EvmLogBackfillBatchSize() uint32 {
	return *c.cfg.LogBackfillBatchSize
}

func (c *ChainScoped) EvmLogPollInterval() time.Duration {
	return c.cfg.LogPollInterval.Duration()
}

func (c *ChainScoped) EvmLogKeepBlocksDepth() uint32 {
	return *c.cfg.LogKeepBlocksDepth
}

func (c *ChainScoped) EvmNonceAutoSync() bool {
	return *c.cfg.NonceAutoSync
}

func (c *ChainScoped) EvmRPCDefaultBatchSize() uint32 {
	return *c.cfg.RPCDefaultBatchSize
}

func (c *ChainScoped) FlagsContractAddress() string {
	if c.cfg.FlagsContractAddress == nil {
		return ""
	}
	return c.cfg.FlagsContractAddress.String()
}

func (c *ChainScoped) GasEstimatorMode() string {
	return *c.cfg.GasEstimator.Mode
}
func (c *ChainScoped) KeySpecificMaxGasPriceWei(addr common.Address) *assets.Wei {
	var keySpecific *assets.Wei
	for i := range c.cfg.KeySpecific {
		ks := c.cfg.KeySpecific[i]
		if ks.Key.Address() == addr {
			keySpecific = ks.GasEstimator.PriceMax
			break
		}
	}

	chainSpecific := c.EvmMaxGasPriceWei()
	if keySpecific != nil && !keySpecific.IsZero() && keySpecific.Cmp(chainSpecific) < 0 {
		return keySpecific
	}

	return c.EvmMaxGasPriceWei()
}

func (c *ChainScoped) LinkContractAddress() string {
	if c.cfg.LinkContractAddress == nil {
		return ""
	}
	return c.cfg.LinkContractAddress.String()
}

func (c *ChainScoped) OperatorFactoryAddress() string {
	if c.cfg.OperatorFactoryAddress == nil {
		return ""
	}
	return c.cfg.OperatorFactoryAddress.String()
}

func (c *ChainScoped) MinIncomingConfirmations() uint32 {
	return *c.cfg.MinIncomingConfirmations
}

func (c *ChainScoped) MinimumContractPayment() *assets.Link {
	return c.cfg.MinContractPayment
}

func (c *ChainScoped) NodeNoNewHeadsThreshold() time.Duration {
	return c.cfg.NoNewHeadsThreshold.Duration()
}

func (c *ChainScoped) NodePollFailureThreshold() uint32 {
	return *c.cfg.NodePool.PollFailureThreshold
}

func (c *ChainScoped) NodePollInterval() time.Duration {
	return c.cfg.NodePool.PollInterval.Duration()
}

func (c *ChainScoped) NodeSelectionMode() string {
	return *c.cfg.NodePool.SelectionMode
}

func (c *ChainScoped) NodeSyncThreshold() uint32 {
	return *c.cfg.NodePool.SyncThreshold
}

func (c *ChainScoped) OCRContractConfirmations() uint16 {
	return *c.cfg.OCR.ContractConfirmations
}

func (c *ChainScoped) OCRContractTransmitterTransmitTimeout() time.Duration {
	return c.cfg.OCR.ContractTransmitterTransmitTimeout.Duration()
}

func (c *ChainScoped) OCRObservationGracePeriod() time.Duration {
	return c.cfg.OCR.ObservationGracePeriod.Duration()
}

func (c *ChainScoped) OCRDatabaseTimeout() time.Duration {
	return c.cfg.OCR.DatabaseTimeout.Duration()
}

func (c *ChainScoped) OCR2AutomationGasLimit() uint32 {
	return *c.cfg.OCR2.Automation.GasLimit
}
