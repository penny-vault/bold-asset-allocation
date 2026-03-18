// Copyright 2021-2026
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/penny-vault/pvbt/asset"
	"github.com/penny-vault/pvbt/data"
	"github.com/penny-vault/pvbt/engine"
	"github.com/penny-vault/pvbt/portfolio"
	"github.com/penny-vault/pvbt/tradecron"
	"github.com/penny-vault/pvbt/universe"
)

//go:embed README.md
var description string

type BoldAssetAllocation struct {
	OffensiveUniverse universe.Universe `pvbt:"offensive-universe" desc:"Offensive (risky) assets to select from" default:"SPY,QQQ,IWM,VGK,EWJ,VWO,VNQ,DBC,GLD,TLT,HYG,LQD" suggest:"BAA-G12=SPY,QQQ,IWM,VGK,EWJ,VWO,VNQ,DBC,GLD,TLT,HYG,LQD|BAA-G4=QQQ,VWO,VEA,BND"`
	DefensiveUniverse universe.Universe `pvbt:"defensive-universe" desc:"Defensive assets for risk-off periods" default:"TIP,DBC,BIL,IEF,TLT,LQD,BND" suggest:"BAA-G12=TIP,DBC,BIL,IEF,TLT,LQD,BND|BAA-G4=TIP,DBC,BIL,IEF,TLT,LQD,BND"`
	CanaryUniverse    universe.Universe `pvbt:"canary-universe" desc:"Canary assets that signal crash protection" default:"SPY,VWO,VEA,BND" suggest:"BAA-G12=SPY,VWO,VEA,BND|BAA-G4=SPY,VWO,VEA,BND"`
	TopO              int               `pvbt:"top-offensive" desc:"Number of top offensive assets to select" default:"6" suggest:"BAA-G12=6|BAA-G4=1"`
	TopD              int               `pvbt:"top-defensive" desc:"Number of top defensive assets to select" default:"3" suggest:"BAA-G12=3|BAA-G4=3"`
	CashTicker        string            `pvbt:"cash-ticker" desc:"Cash asset ticker used as absolute momentum floor for defensive assets" default:"BIL" suggest:"BAA-G12=BIL|BAA-G4=BIL"`
}

func (s *BoldAssetAllocation) Name() string {
	return "Bold Asset Allocation"
}

func (s *BoldAssetAllocation) Setup(eng *engine.Engine) {
	tc, err := tradecron.New("@monthend", tradecron.MarketHours{Open: 930, Close: 1600})
	if err != nil {
		panic(err)
	}

	eng.Schedule(tc)
	eng.SetBenchmark(eng.Asset("VFINX"))
}

func (s *BoldAssetAllocation) Describe() engine.StrategyDescription {
	return engine.StrategyDescription{
		ShortCode:   "baa",
		Description: description,
		Source:      "https://papers.ssrn.com/sol3/papers.cfm?abstract_id=4166845",
		Version:     "1.0.0",
		VersionDate: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
	}
}

func (s *BoldAssetAllocation) Compute(ctx context.Context, eng *engine.Engine, strategyPortfolio portfolio.Portfolio) error {
	// 1. Fetch 12-month window of monthly close prices for all universes.
	offensiveDF, err := s.OffensiveUniverse.Window(ctx, portfolio.Months(12), data.MetricClose)
	if err != nil {
		return fmt.Errorf("failed to fetch offensive universe prices: %w", err)
	}

	defensiveDF, err := s.DefensiveUniverse.Window(ctx, portfolio.Months(12), data.MetricClose)
	if err != nil {
		return fmt.Errorf("failed to fetch defensive universe prices: %w", err)
	}

	canaryDF, err := s.CanaryUniverse.Window(ctx, portfolio.Months(12), data.MetricClose)
	if err != nil {
		return fmt.Errorf("failed to fetch canary universe prices: %w", err)
	}

	// 2. Downsample to monthly frequency.
	offensiveMonthly := offensiveDF.Downsample(data.Monthly).Last()
	defensiveMonthly := defensiveDF.Downsample(data.Monthly).Last()
	canaryMonthly := canaryDF.Downsample(data.Monthly).Last()

	// Need at least 13 rows for Pct(12) and Rolling(13).Mean() to produce valid values.
	if offensiveMonthly.Len() < 13 || defensiveMonthly.Len() < 13 || canaryMonthly.Len() < 13 {
		return nil
	}

	// 3. Compute canary momentum (fast 13612W) for crash protection.
	//    Note: 13612W intentionally omits the /4 divisor used in DAA's momentum12631.
	//    BAA only checks the sign, so the scale does not matter.
	canaryMom := momentum13612W(canaryMonthly)
	canaryMom = canaryMom.Drop(math.NaN()).Last()

	if canaryMom.Len() == 0 {
		return nil
	}

	canaryMom.Annotate(strategyPortfolio)

	// Check if ANY canary asset has negative absolute momentum (B=1).
	anyBad := false

	for _, a := range canaryMom.AssetList() {
		if canaryMom.Value(a, data.MetricClose) < 0 {
			anyBad = true
			break
		}
	}

	ts := eng.CurrentDate().Unix()

	regime := "offensive"
	if anyBad {
		regime = "defensive"
	}

	strategyPortfolio.Annotate(ts, "regime", regime)

	// 4. Compute SMA(12) relative momentum for offensive and defensive universes.
	offensiveMom := momentumSMA12(offensiveMonthly)
	offensiveMom = offensiveMom.Drop(math.NaN()).Last()

	defensiveMom := momentumSMA12(defensiveMonthly)
	defensiveMom = defensiveMom.Drop(math.NaN()).Last()

	if offensiveMom.Len() == 0 || defensiveMom.Len() == 0 {
		return nil
	}

	offensiveMom.Annotate(strategyPortfolio)
	defensiveMom.Annotate(strategyPortfolio)

	// Helper: sort assets by momentum score descending.
	type assetScore struct {
		a     asset.Asset
		score float64
	}

	sortByScore := func(mom *data.DataFrame) []assetScore {
		var scores []assetScore
		for _, a := range mom.AssetList() {
			scores = append(scores, assetScore{a: a, score: mom.Value(a, data.MetricClose)})
		}

		sort.Slice(scores, func(i, j int) bool {
			return scores[i].score > scores[j].score
		})

		return scores
	}

	members := make(map[asset.Asset]float64)

	var justification string

	if !anyBad {
		// OFFENSIVE: select TopO assets by SMA(12), equal weight.
		scores := sortByScore(offensiveMom)

		topO := s.TopO
		if topO > len(scores) {
			topO = len(scores)
		}

		weight := 1.0 / float64(topO)
		for _, sc := range scores[:topO] {
			members[sc.a] = weight
		}

		justification = fmt.Sprintf("offensive: top %d by SMA(12)", topO)
	} else {
		// DEFENSIVE: select TopD assets by SMA(12), but replace any with
		// momentum < BIL's momentum with BIL (absolute momentum filter).
		scores := sortByScore(defensiveMom)

		topD := s.TopD
		if topD > len(scores) {
			topD = len(scores)
		}

		// Find BIL's SMA(12) momentum as the absolute momentum floor.
		cashAsset := eng.Asset(s.CashTicker)
		cashMom := math.Inf(-1)

		for _, sc := range scores {
			if sc.a == cashAsset {
				cashMom = sc.score
				break
			}
		}

		weight := 1.0 / float64(topD)
		cashWeight := 0.0

		for _, sc := range scores[:topD] {
			if sc.score >= cashMom || sc.a == cashAsset {
				members[sc.a] += weight
			} else {
				// Replace with cash.
				cashWeight += weight
			}
		}

		if cashWeight > 0 {
			members[cashAsset] += cashWeight
		}

		justification = fmt.Sprintf("defensive: top %d by SMA(12), cash=%s", topD, s.CashTicker)
	}

	strategyPortfolio.Annotate(ts, "justification", justification)

	allocation := portfolio.Allocation{
		Date:          eng.CurrentDate(),
		Members:       members,
		Justification: justification,
	}

	if err := strategyPortfolio.RebalanceTo(ctx, allocation); err != nil {
		return fmt.Errorf("rebalance failed: %w", err)
	}

	return nil
}

// momentum13612W computes the fast 13612W absolute momentum:
//
//	12*RET(1) + 4*RET(3) + 2*RET(6) + 1*RET(12)
//
// where RET(n) = p0/pn - 1 (n-month return).
// This intentionally omits the /4 divisor used in DAA's momentum12631 --
// BAA only checks the sign of canary momentum, so the scale does not matter.
func momentum13612W(df *data.DataFrame) *data.DataFrame {
	ret1 := df.Pct(1).MulScalar(12)
	ret3 := df.Pct(3).MulScalar(4)
	ret6 := df.Pct(6).MulScalar(2)
	ret12 := df.Pct(12)

	return ret1.Add(ret3).Add(ret6).Add(ret12)
}

// momentumSMA12 computes the slow SMA(12) relative momentum:
//
//	p0 / average(p0, p1, ..., p12) - 1
//
// This is the current price divided by the 13-period simple moving average, minus 1.
func momentumSMA12(df *data.DataFrame) *data.DataFrame {
	sma13 := df.Rolling(13).Mean()
	return df.Div(sma13).AddScalar(-1)
}
