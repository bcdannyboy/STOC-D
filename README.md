# STOC'D: Stochastic Trade Optimization for Credit Derivatives

![STOC'D Logo](./stocd.webp)

STOC'D (Stochastic Trade Optimization for Credit Derivatives) is an advanced options trading analysis tool that employs various stochastic models and volatility estimation techniques to identify optimal credit spread opportunities.

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

6. **Heston Stochastic Volatility**: Estimates volatility using the Heston model with mean-reverting stochastic volatility.

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

### Expand Risk Management Tools

- [ ] **Expected Shortfall (ES) Implementation**:
  - [ ] Develop tools to calculate Expected Shortfall (also known as Conditional VaR) at various confidence levels, providing a more comprehensive view of potential losses beyond VaR.

- [ ] **Stress Testing Scenarios**:
  - [ ] **Monte Carlo-based Stress Testing**: Design stress testing frameworks that simulate extreme market conditions using Monte Carlo methods to assess the robustness of trading strategies under stress scenarios.
  - [ ] **Liquidity Risk Assessment**:
    - [ ] **Bid-Ask Spread Analysis**: Develop a tool to analyze the bid-ask spread for options, particularly during high volatility periods, to assess liquidity risk and its impact on trade execution and pricing.

### Integrate Advanced Volatility Modeling

- [ ] **SABR Model Implementation**: Stochastic Alpha Beta Rho (SABR) model for volatility modeling, offering a more flexible framework for capturing the dynamics of implied volatility surfaces.
- [ ] **Bates Model Implementation**: Integrate the Bates model, which combines stochastic volatility and jumps, to capture the complex dynamics of asset prices and improve option pricing accuracy.

### Expand One-Dimensional Stochastic Models

- [ ] **Longstaff-Schwartz Method**: Implement the Longstaff-Schwartz method for American option pricing, allowing for more accurate pricing of options with early exercise features.
- [ ] **Variance Gamma Model**: Implement the Variance Gamma model for more accurate pricing of options, particularly in markets with heavy tails or skewed distributions.
- [ ] **Normal Inverse Gaussian (NIG) Model**: Integrate the NIG model to account for skewness and kurtosis in asset returns, offering a more nuanced approach to option pricing and risk management.
- [ ] **Enhanced Model Calibrations**: Improve the calibration of existing models, ensuring that they accurately reflect market conditions and historical data, enhancing the reliability of simulations and pricing.

### Develop Multi-Dimensional Stochastic Modeling

- [ ] **Lévy Copulas for Multi-Asset Correlation Modeling**: Implement Lévy copulas to model the dependency structures between multiple assets, allowing for more sophisticated multi-asset portfolio simulations and risk assessments.

- [ ] **Extended Monte Carlo for Multiple Assets**:
  - [ ] **Correlated Asset Price Simulations**: Develop algorithms for simulating correlated price paths across multiple assets, providing insights into the joint behavior of assets in a portfolio.
  - [ ] **Multi-Asset Path Generation Algorithms**: Create advanced path generation techniques for scenarios involving multiple assets, enhancing the simulation of complex trading strategies.

- [ ] **Multi-Asset Option Pricing Models**:
  - [ ] **Basket Option Pricing**: Implement models to price basket options, where the payoff depends on the performance of a group of assets, expanding the tool's capabilities to handle more complex options strategies.

### Improve Greeks Calculations

- [ ] **Vomma and Vanna Calculations**: Add support for calculating higher-order Greeks like vomma (sensitivity of vega to volatility) and vanna (sensitivity of delta to volatility), providing deeper insights into options risk management.

- [ ] **Numerical Methods for Higher-Order Greeks**: Implement robust numerical techniques for accurately calculating higher-order Greeks, essential for sophisticated hedging and risk management strategies.

### Implement Advanced Hedging Strategies

- [ ] **Options Greeks Hedging**:
  - [ ] **Delta-Gamma Hedging Algorithms**: Develop algorithms to hedge both delta and gamma, ensuring that portfolios are protected against small and large movements in the underlying asset.
  - [ ] **Vega Hedging Strategies**: Create strategies to hedge against volatility risk, ensuring that portfolios are less sensitive to changes in implied volatility.

- [ ] **Mean-Variance Hedging**:
  - [ ] **Quadratic Hedging Techniques**: Implement mean-variance hedging approaches to minimize the variance of portfolio returns, improving risk-adjusted performance.
  - [ ] **Hedging Performance Metrics**: Develop metrics to evaluate the effectiveness of hedging strategies, providing traders with actionable insights into their hedging performance.

- [ ] **Dynamic Hedging Strategies**:
  - [ ] **Adaptive Hedging Based on Market Conditions**: Implement dynamic hedging strategies that adjust based on real-time market conditions, improving the resilience and adaptability of trading strategies.

### Improve Spread Identification and Analysis

- [ ] **More Spread Strategies**:
  - [ ] **Iron Condor Identification Algorithm**: Introduce algorithms to identify iron condor opportunities, expanding the range of spread strategies available to traders.
  - [ ] **Butterfly Spread Analysis**: Develop tools to analyze butterfly spreads, providing detailed insights into their potential risks and rewards.

- [ ] **Enhanced Spread Scoring System**:
  - [ ] **Dynamic Weighting Based on Market Conditions**: Refine the spread scoring system to dynamically adjust weightings based on current market conditions, ensuring that the most relevant factors are emphasized in scoring.

### Develop Comprehensive Portfolio Management

- [ ] **Position Management**:
  - [ ] **Add/Close Position Functions with P&L Tracking**: Implement functionalities for managing open positions, including tools to track profit and loss (P&L) in real-time.
  - [ ] **Position Sizing Algorithms**: Develop algorithms for optimal position sizing, helping traders manage risk and maximize returns.

- [ ] **Historical Tracking and Analysis**:
  - [ ] **Time Series Analysis of Portfolio Performance**: Implement tools for analyzing portfolio performance over time, using time series analysis to identify trends and make data-driven adjustments.

- [ ] **Portfolio-Level Tools**:
  - [ ] **Portfolio VaR and ES Calculations**: Extend VaR and ES calculations to the portfolio level, providing a holistic view of risk across all positions.
  - [ ] **Portfolio Optimization Algorithms**: Develop algorithms to optimize portfolios based on multiple criteria, including risk, return, and capital allocation.
