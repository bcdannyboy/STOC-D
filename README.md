# STOC'D: Stochastic Optimization for Credit Spread Decision Making

![STOC'D Logo](./stocd.webp)

STOC'D (Stochastic Optimization for Credit Spread Decision Making) is an advanced options trading analysis tool that employs various stochastic models and volatility estimation techniques to identify optimal credit spread opportunities.

## Table of Contents

1. [Introduction](#introduction)
2. [Features](#features)
3. [Installation](#installation)
4. [Usage](#usage)
5. [Technical Details](#technical-details)
   - [Data Fetching](#data-fetching)
   - [Volatility Estimation](#volatility-estimation)
   - [Stochastic Models](#stochastic-models)
   - [Option Pricing](#option-pricing)
   - [Spread Identification](#spread-identification)
   - [Probability Calculation](#probability-calculation)
   - [Risk Assessment](#risk-assessment)
   - [Scoring and Ranking](#scoring-and-ranking)
6. [Future Enhancements](#future-enhancements)

## Introduction

STOC'D is designed to assist traders in making informed decisions about credit spread strategies. It combines historical data analysis, options chain information, and advanced stochastic models to provide comprehensive insights into potential trades.

## Features

- Fetch and analyze historical price data and options chains
- Implement multiple volatility estimation techniques
- Utilize advanced stochastic models for price simulation
- Identify optimal credit spread opportunities
- Perform Monte Carlo simulations for probability estimation
- Calculate Value at Risk (VaR) for risk assessment
- Score and rank spread opportunities based on multiple factors

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/bcdannyboy/STOC-D.git
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Set up your Tradier API key in a `.env` file:
   ```
   TRADIER_KEY=your_api_key_here
   ```

## Usage

Run the main program:

```
go run main.go
```

This will fetch options data, analyze potential credit spreads, and output the results.

## Technical Details

### Data Fetching

- Utilizes the Tradier API to fetch historical price data, options chains, and price statistics
- Implements functions to retrieve quotes, options expirations, and full options chains

### Volatility Estimation

1. **Yang-Zhang Volatility**: Calculates volatility using opening, closing, high, and low prices, accounting for overnight jumps.

2. **Rogers-Satchell Volatility**: Estimates volatility using high, low, opening, and closing prices, independent of price drift.

3. **Local Volatility Surface**: Creates a volatility surface based on option prices across different strikes and expirations.

4. **Implied Volatility**: Calculates implied volatility for individual options using the Black-Scholes-Merton model.

5. **Historical Volatility**: Computes historical volatility from past price data.

### Stochastic Models

1. **Black-Scholes-Merton (BSM) Model**: Implements the classic option pricing model for European options.

2. **Heston Stochastic Volatility Model**: Simulates price paths with mean-reverting stochastic volatility.

3. **Merton Jump Diffusion Model**: Incorporates jumps in the asset price process.

4. **Kou Jump Diffusion Model**: Uses a double exponential distribution for jump sizes.

5. **CGMY (Carr-Geman-Madan-Yor) Model**: Implements a tempered stable process allowing for infinite activity of small jumps and finite activity of large jumps.

### Option Pricing

- Implements the Black-Scholes-Merton formula for European option pricing
- Calculates option Greeks (Delta, Gamma, Theta, Vega, Rho)
- Computes implied volatility using numerical methods (Newton-Raphson)

### Spread Identification

- Identifies potential Bull Put and Bear Call spread opportunities
- Filters spreads based on minimum Days to Expiration (DTE) and maximum DTE
- Calculates spread credit, max risk, and return on risk (ROR)

### Probability Calculation

- Performs Monte Carlo simulations using various stochastic models
- Estimates probability of profit for identified spreads
- Incorporates multiple volatility estimates in simulations

### Risk Assessment

- Calculates Value at Risk (VaR) at 95% and 99% confidence levels
- Computes potential profit/loss for simulated price paths

### Scoring and Ranking

- Implements a composite scoring system considering probability of profit, VaR, and trading volume
- Normalizes individual factors to create a balanced score
- Ranks spread opportunities based on the composite score

## Future Enhancements

These enhancements are in no particular order

- [ ] Expand One-Dimensional Stochastic Models
  - [ ] Implement Variance Gamma model
  - [ ] Add Normal Inverse Gaussian model
  - [ ] Enhance existing model calibrations

- [ ] Develop Multi-Dimensional Stochastic Modeling
  - [ ] Implement Lévy Copulas for multi-asset correlation modeling
  - [ ] Extend Monte Carlo for multiple assets
    - [ ] Implement correlated asset price simulations
    - [ ] Develop multi-asset path generation algorithms
  - [ ] Create multi-asset option pricing models
    - [ ] Implement basket option pricing

- [ ] Integrate Advanced Volatility Modeling
  - [ ] Implement GARCH models
    - [ ] Develop GARCH(1,1) and EGARCH models
    - [ ] Create volatility forecasting functions
  - [ ] Develop regime-switching models
    - [ ] Implement Markov-switching GARCH
    - [ ] Create hidden Markov model for volatility regimes
  - [ ] Enhance local volatility calculations
    - [ ] Improve interpolation techniques for vol surface
    - [ ] Develop more robust fitting algorithms

- [ ] Improve Greeks calculations
  - [ ] Add vomma and vanna calculations
  - [ ] Implement numerical methods for higher-order Greeks

- [ ] Expand Risk Management Tools
  - [ ] Implement Expected Shortfall (ES)
  - [ ] Develop stress testing scenarios
    - [ ] Create Monte Carlo-based stress testing
  - [ ] Add liquidity risk assessment
    - [ ] Develop bid-ask spread analysis for options

- [ ] Improve Spread Identification and Analysis
  - [ ] Implement more spread strategies
    - [ ] Add iron condor identification algorithm
    - [ ] Develop butterfly spread analysis
  - [ ] Enhance spread scoring system
    - [ ] Develop dynamic weighting based on market conditions

- [ ] Develop Comprehensive Portfolio Management
  - [ ] Implement position management
    - [ ] Develop add/close position functions with P&L tracking
    - [ ] Create position sizing algorithms
  - [ ] Develop historical tracking and analysis
    - [ ] Implement time series analysis of portfolio performance
  - [ ] Create portfolio-level tools
    - [ ] Develop portfolio VaR and ES calculations
    - [ ] Implement portfolio optimization algorithms

- [ ] Implement Advanced Hedging Strategies
  - [ ] Develop Options Greeks hedging
    - [ ] Implement delta-gamma hedging algorithms
    - [ ] Create vega hedging strategies
  - [ ] Implement Mean-Variance hedging
    - [ ] Develop quadratic hedging techniques
    - [ ] Create hedging performance metrics
  - [ ] Create dynamic hedging strategies
    - [ ] Implement adaptive hedging based on market conditions
