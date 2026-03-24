# Bold Asset Allocation

The **Bold Asset Allocation** strategy was developed by [Wouter Keller](https://papers.ssrn.com/sol3/cf_dev/AbsByAuth.cfm?per_id=1935527). It is based on his paper: [Relative and Absolute Momentum in Times of Rising/Low Yields: Bold Asset Allocation (BAA)](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=4166845). BAA combines slow relative momentum (SMA12) with fast absolute momentum (13612W) and uses a canary universe for crash protection. It comes in two variants: balanced (G12, Top6) and aggressive (G4, Top1).

## Rules

The strategy has three groups of assets:

**BAA-G12 (Balanced):**
- **Offensive**: SPY, QQQ, IWM, VGK, EWJ, VWO, VNQ, DBC, GLD, TLT, HYG, LQD
- **Defensive**: TIP, DBC, BIL, IEF, TLT, LQD, BND
- **Canary**: SPY, VWO, VEA, BND

**BAA-G4 (Aggressive):**
- **Offensive**: QQQ, VWO, VEA, BND
- **Defensive**: TIP, DBC, BIL, IEF, TLT, LQD, BND (same as balanced)
- **Canary**: SPY, VWO, VEA, BND (same as balanced)

1. On the last trading day of the month, compute two types of momentum:
   - **13612W (fast, absolute)**: 12 * (p0/p1 - 1) + 4 * (p0/p3 - 1) + 2 * (p0/p6 - 1) + (p0/p12 - 1)
   - **SMA(12) (slow, relative)**: p0 / average(p0, p1, ..., p12) - 1
2. Compute 13612W momentum for all canary assets. If ANY canary asset has negative 13612W momentum, switch to the defensive universe (B=1).
3. **Offensive allocation** (no bad canary assets):
   - Rank offensive assets by SMA(12) relative momentum
   - Select Top 6 (balanced) or Top 1 (aggressive), equal weight
4. **Defensive allocation** (any bad canary asset):
   - Rank defensive assets by SMA(12) relative momentum
   - Select Top 3, equal weight
   - If any selected asset has SMA(12) momentum less than BIL's SMA(12) momentum, replace that asset's allocation with BIL
5. Hold all positions until the close of the following month.

## Assets Typically Held

| Ticker | Name                                                | Sector                              |
| ------ | --------------------------------------------------- | ----------------------------------- |
| SPY    | SPDR S&P 500 ETF                                    | Equity, U.S., Large Cap             |
| QQQ    | Invesco QQQ                                         | Equity, U.S., Large Cap             |
| IWM    | iShares Russell 2000 ETF                            | Equity, U.S., Small Cap             |
| VGK    | Vanguard FTSE Europe ETF                            | Equity, Europe, Large Cap           |
| EWJ    | iShares MSCI Japan ETF                              | Equity, Japan, Large Cap            |
| VWO    | Vanguard FTSE Emerging Markets ETF                  | Equity, Emerging Markets            |
| VEA    | Vanguard FTSE Developed Markets ETF                 | Equity, Developed Markets           |
| VNQ    | Vanguard Real Estate Index Fund ETF                 | Real Estate, U.S.                   |
| DBC    | Invesco DB Commodity Index Tracking Fund            | Commodity, Diversified              |
| GLD    | SPDR Gold Trust                                     | Commodity, Gold                     |
| TLT    | iShares 20+ Year Treasury Bond ETF                  | Bond, U.S., Long-Term               |
| HYG    | iShares iBoxx $ High Yield Corporate Bond ETF       | Bond, U.S., High Yield              |
| LQD    | iShares iBoxx $ Investment Grade Corporate Bond ETF | Bond, U.S., Investment Grade        |
| TIP    | iShares TIPS Bond ETF                               | Bond, U.S., Inflation-Protected     |
| BIL    | SPDR Bloomberg 1-3 Month T-Bill ETF                 | Bond, U.S., Short-Term              |
| IEF    | iShares 7-10 Year Treasury Bond ETF                 | Bond, U.S., Intermediate-Term       |
| BND    | Vanguard Total Bond Market ETF                      | Bond, U.S., Aggregate               |
