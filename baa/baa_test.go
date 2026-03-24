package baa_test

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/penny-vault/bold-asset-allocation/baa"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
)

var _ = Describe("BoldAssetAllocation", func() {
	var (
		ctx       context.Context
		snap      *data.SnapshotProvider
		nyc       *time.Location
		startDate time.Time
		endDate   time.Time
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		nyc, err = time.LoadLocation("America/New_York")
		Expect(err).NotTo(HaveOccurred())

		snap, err = data.NewSnapshotProvider("testdata/snapshot.db")
		Expect(err).NotTo(HaveOccurred())

		startDate = time.Date(2024, 6, 1, 0, 0, 0, 0, nyc)
		endDate = time.Date(2026, 3, 1, 0, 0, 0, 0, nyc)
	})

	AfterEach(func() {
		if snap != nil {
			snap.Close()
		}
	})

	runBacktest := func() portfolio.Portfolio {
		strategy := &baa.BoldAssetAllocation{}
		acct := portfolio.New(
			portfolio.WithCash(100000, startDate),
			portfolio.WithAllMetrics(),
		)

		eng := engine.New(strategy,
			engine.WithDataProvider(snap),
			engine.WithAssetProvider(snap),
			engine.WithAccount(acct),
		)

		result, err := eng.Backtest(ctx, startDate, endDate)
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	It("produces expected returns and risk metrics", func() {
		result := runBacktest()

		summary, err := result.Summary()
		Expect(err).NotTo(HaveOccurred())
		Expect(summary.TWRR).To(BeNumerically("~", 0.2205, 0.01))
		Expect(summary.MaxDrawdown).To(BeNumerically(">", -0.12), "max drawdown should be better than -12%")

		Expect(result.Value()).To(BeNumerically("~", 122046, 500))
	})

	It("trades both offensive and defensive assets", func() {
		result := runBacktest()
		txns := result.Transactions()

		tickers := map[string]bool{}
		for _, t := range txns {
			if t.Type == portfolio.BuyTransaction || t.Type == portfolio.SellTransaction {
				tickers[t.Asset.Ticker] = true
			}
		}

		// Offensive assets
		Expect(tickers).To(HaveKey("SPY"))
		Expect(tickers).To(HaveKey("QQQ"))
		Expect(tickers).To(HaveKey("GLD"))
		Expect(tickers).To(HaveKey("VGK"))
		Expect(tickers).To(HaveKey("EWJ"))
		Expect(tickers).To(HaveKey("VWO"))
		Expect(tickers).To(HaveKey("IWM"))

		// Defensive assets
		Expect(tickers).To(HaveKey("BIL"))
		Expect(tickers).To(HaveKey("IEF"))
		Expect(tickers).To(HaveKey("TIP"))
		Expect(tickers).To(HaveKey("DBC"))
	})

	It("produces the expected trade sequence", func() {
		result := runBacktest()
		txns := result.Transactions()

		type trade struct {
			date   string
			txType portfolio.TransactionType
			ticker string
		}

		var trades []trade
		for _, t := range txns {
			if t.Type == portfolio.BuyTransaction || t.Type == portfolio.SellTransaction {
				trades = append(trades, trade{
					date:   t.Date.In(nyc).Format("2006-01-02"),
					txType: t.Type,
					ticker: t.Asset.Ticker,
				})
			}
		}

		// Sort trades within each date by type (sell before buy) then ticker
		// to make the comparison deterministic regardless of map iteration order.
		sort.SliceStable(trades, func(i, j int) bool {
			if trades[i].date != trades[j].date {
				return trades[i].date < trades[j].date
			}
			if trades[i].txType != trades[j].txType {
				return trades[i].txType > trades[j].txType // sell before buy
			}
			return trades[i].ticker < trades[j].ticker
		})

		expected := []trade{
			// 2024-06-28: defensive (canary negative) -> BIL, LQD
			{"2024-06-28", portfolio.BuyTransaction, "BIL"},
			{"2024-06-28", portfolio.BuyTransaction, "LQD"},
			// 2024-07-31: offensive -> top 6
			{"2024-07-31", portfolio.SellTransaction, "BIL"},
			{"2024-07-31", portfolio.SellTransaction, "LQD"},
			{"2024-07-31", portfolio.BuyTransaction, "EWJ"},
			{"2024-07-31", portfolio.BuyTransaction, "GLD"},
			{"2024-07-31", portfolio.BuyTransaction, "IWM"},
			{"2024-07-31", portfolio.BuyTransaction, "QQQ"},
			{"2024-07-31", portfolio.BuyTransaction, "SPY"},
			{"2024-07-31", portfolio.BuyTransaction, "VNQ"},
			// 2024-08-30: offensive rebalance
			{"2024-08-30", portfolio.SellTransaction, "EWJ"},
			{"2024-08-30", portfolio.SellTransaction, "VNQ"},
			{"2024-08-30", portfolio.BuyTransaction, "IWM"},
			{"2024-08-30", portfolio.BuyTransaction, "QQQ"},
			{"2024-08-30", portfolio.BuyTransaction, "VGK"},
			// 2024-09-30: offensive rebalance
			{"2024-09-30", portfolio.SellTransaction, "GLD"},
			{"2024-09-30", portfolio.SellTransaction, "VGK"},
			{"2024-09-30", portfolio.BuyTransaction, "IWM"},
			{"2024-09-30", portfolio.BuyTransaction, "VWO"},
			// 2024-10-31: defensive -> BIL, LQD
			{"2024-10-31", portfolio.SellTransaction, "GLD"},
			{"2024-10-31", portfolio.SellTransaction, "IWM"},
			{"2024-10-31", portfolio.SellTransaction, "QQQ"},
			{"2024-10-31", portfolio.SellTransaction, "SPY"},
			{"2024-10-31", portfolio.SellTransaction, "VNQ"},
			{"2024-10-31", portfolio.SellTransaction, "VWO"},
			{"2024-10-31", portfolio.BuyTransaction, "BIL"},
			{"2024-10-31", portfolio.BuyTransaction, "LQD"},
			// 2024-11-29: offensive -> top 6
			{"2024-11-29", portfolio.SellTransaction, "BIL"},
			{"2024-11-29", portfolio.SellTransaction, "LQD"},
			{"2024-11-29", portfolio.BuyTransaction, "GLD"},
			{"2024-11-29", portfolio.BuyTransaction, "IWM"},
			{"2024-11-29", portfolio.BuyTransaction, "QQQ"},
			{"2024-11-29", portfolio.BuyTransaction, "SPY"},
			{"2024-11-29", portfolio.BuyTransaction, "VNQ"},
			{"2024-11-29", portfolio.BuyTransaction, "VWO"},
			// 2024-12-31: defensive -> BIL (all cash)
			{"2024-12-31", portfolio.SellTransaction, "GLD"},
			{"2024-12-31", portfolio.SellTransaction, "IWM"},
			{"2024-12-31", portfolio.SellTransaction, "QQQ"},
			{"2024-12-31", portfolio.SellTransaction, "SPY"},
			{"2024-12-31", portfolio.SellTransaction, "VNQ"},
			{"2024-12-31", portfolio.SellTransaction, "VWO"},
			{"2024-12-31", portfolio.BuyTransaction, "BIL"},
			// 2025-01-31: offensive -> top 6
			{"2025-01-31", portfolio.SellTransaction, "BIL"},
			{"2025-01-31", portfolio.BuyTransaction, "GLD"},
			{"2025-01-31", portfolio.BuyTransaction, "HYG"},
			{"2025-01-31", portfolio.BuyTransaction, "IWM"},
			{"2025-01-31", portfolio.BuyTransaction, "QQQ"},
			{"2025-01-31", portfolio.BuyTransaction, "SPY"},
			{"2025-01-31", portfolio.BuyTransaction, "VWO"},
			// 2025-02-28: offensive rebalance
			{"2025-02-28", portfolio.SellTransaction, "HYG"},
			{"2025-02-28", portfolio.SellTransaction, "IWM"},
			{"2025-02-28", portfolio.SellTransaction, "VWO"},
			{"2025-02-28", portfolio.BuyTransaction, "QQQ"},
			{"2025-02-28", portfolio.BuyTransaction, "VGK"},
			{"2025-02-28", portfolio.BuyTransaction, "VNQ"},
			// 2025-03-31: defensive -> DBC, IEF, TIP
			{"2025-03-31", portfolio.SellTransaction, "GLD"},
			{"2025-03-31", portfolio.SellTransaction, "HYG"},
			{"2025-03-31", portfolio.SellTransaction, "QQQ"},
			{"2025-03-31", portfolio.SellTransaction, "SPY"},
			{"2025-03-31", portfolio.SellTransaction, "VGK"},
			{"2025-03-31", portfolio.SellTransaction, "VNQ"},
			{"2025-03-31", portfolio.BuyTransaction, "DBC"},
			{"2025-03-31", portfolio.BuyTransaction, "IEF"},
			{"2025-03-31", portfolio.BuyTransaction, "TIP"},
			// 2025-04-30: defensive rebalance -> BND, IEF, TIP
			{"2025-04-30", portfolio.SellTransaction, "DBC"},
			{"2025-04-30", portfolio.SellTransaction, "IEF"},
			{"2025-04-30", portfolio.SellTransaction, "TIP"},
			{"2025-04-30", portfolio.BuyTransaction, "BND"},
			// 2025-05-30: defensive -> all cash (BIL)
			{"2025-05-30", portfolio.SellTransaction, "BND"},
			{"2025-05-30", portfolio.SellTransaction, "IEF"},
			{"2025-05-30", portfolio.SellTransaction, "TIP"},
			{"2025-05-30", portfolio.BuyTransaction, "BIL"},
			// 2025-06-30: offensive -> top 6
			{"2025-06-30", portfolio.SellTransaction, "BIL"},
			{"2025-06-30", portfolio.BuyTransaction, "EWJ"},
			{"2025-06-30", portfolio.BuyTransaction, "GLD"},
			{"2025-06-30", portfolio.BuyTransaction, "QQQ"},
			{"2025-06-30", portfolio.BuyTransaction, "SPY"},
			{"2025-06-30", portfolio.BuyTransaction, "VGK"},
			{"2025-06-30", portfolio.BuyTransaction, "VWO"},
			// 2025-07-31: offensive rebalance (minor)
			{"2025-07-31", portfolio.SellTransaction, "VWO"},
			{"2025-07-31", portfolio.BuyTransaction, "EWJ"},
			{"2025-07-31", portfolio.BuyTransaction, "VGK"},
			{"2025-07-31", portfolio.BuyTransaction, "VWO"},
			// 2025-08-29: minor rebalance
			{"2025-08-29", portfolio.SellTransaction, "EWJ"},
			// 2025-09-30: rebalance
			{"2025-09-30", portfolio.SellTransaction, "GLD"},
			{"2025-09-30", portfolio.BuyTransaction, "EWJ"},
			{"2025-09-30", portfolio.BuyTransaction, "VGK"},
			// 2025-10-31: rebalance
			{"2025-10-31", portfolio.SellTransaction, "EWJ"},
			{"2025-10-31", portfolio.BuyTransaction, "VGK"},
			{"2025-10-31", portfolio.BuyTransaction, "VWO"},
			// 2025-11-28: offensive rotation
			{"2025-11-28", portfolio.SellTransaction, "GLD"},
			{"2025-11-28", portfolio.SellTransaction, "SPY"},
			{"2025-11-28", portfolio.SellTransaction, "VGK"},
			{"2025-11-28", portfolio.BuyTransaction, "EWJ"},
			{"2025-11-28", portfolio.BuyTransaction, "IWM"},
			{"2025-11-28", portfolio.BuyTransaction, "VWO"},
			// 2025-12-31: rebalance
			{"2025-12-31", portfolio.SellTransaction, "GLD"},
			{"2025-12-31", portfolio.SellTransaction, "VGK"},
			{"2025-12-31", portfolio.BuyTransaction, "EWJ"},
			{"2025-12-31", portfolio.BuyTransaction, "IWM"},
			{"2025-12-31", portfolio.BuyTransaction, "QQQ"},
			{"2025-12-31", portfolio.BuyTransaction, "VWO"},
			// 2026-01-30: offensive rotation
			{"2026-01-30", portfolio.SellTransaction, "GLD"},
			{"2026-01-30", portfolio.SellTransaction, "QQQ"},
			{"2026-01-30", portfolio.BuyTransaction, "DBC"},
			{"2026-01-30", portfolio.BuyTransaction, "VGK"},
			{"2026-01-30", portfolio.BuyTransaction, "VWO"},
			// 2026-02-27: rebalance
			{"2026-02-27", portfolio.SellTransaction, "EWJ"},
			{"2026-02-27", portfolio.SellTransaction, "GLD"},
			{"2026-02-27", portfolio.BuyTransaction, "DBC"},
			{"2026-02-27", portfolio.BuyTransaction, "IWM"},
			{"2026-02-27", portfolio.BuyTransaction, "VGK"},
			{"2026-02-27", portfolio.BuyTransaction, "VWO"},
		}

		Expect(trades).To(HaveLen(len(expected)))
		for i, exp := range expected {
			Expect(trades[i].date).To(Equal(exp.date), "trade %d date", i)
			Expect(trades[i].txType).To(Equal(exp.txType), "trade %d type", i)
			Expect(trades[i].ticker).To(Equal(exp.ticker), "trade %d ticker", i)
		}
	})
})
