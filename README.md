# STOC'D: Stochastic Trade Optimization for Credit Derivatives

![STOC'D Logo](./stocd.webp)

STOC'D (Stochastic Trade Optimization for Credit Derivatives) is an advanced options trading analysis tool that employs various stochastic models and volatility estimation techniques to identify optimal credit spread opportunities.

## Table of Contents

1. [Introduction](#introduction)
2. [Features](#features)
3. [Installation](#installation)
4. [Usage](#usage)
5. [Technical Details](#technical-details)
   - [Computational Complexity](#computational-complexity)
   - [Data Fetching](#data-fetching)
   - [Volatility Estimation](#volatility-estimation)
   - [Probabilistic Models](#probabilistic-models)
   - [Option Pricing](#option-pricing)
   - [Spread Identification](#spread-identification)
   - [Probability Calculation](#probability-calculation)
   - [Risk Assessment](#risk-assessment)
   - [Scoring and Ranking](#scoring-and-ranking)
6. [Future Enhancements](#future-enhancements)

## Introduction

STOC'D is designed to assist traders in making informed decisions about credit spread strategies. It combines historical data analysis, options chain information, and advanced stochastic models to provide comprehensive insights into potential trades.

## Features

- **Options Data Analysis**: Fetches historical price data, options chains, and price statistics from the Tradier API.
- **Volatility Estimation**: Calculates volatility using various models, including Yang-Zhang, Rogers-Satchell, and Heston.
- **Stochastic Models**: Implements Black-Scholes-Merton, Heston, Merton, Kou, and CGMY models for option pricing and simulation.
- **Option Pricing**: Calculates option prices, Greeks, and implied volatility using the Black-Scholes-Merton model.
- **Spread Identification**: Identifies potential Bull Put and Bear Call spread opportunities based on user-defined criteria.
- **Probability Calculation**: Estimates probability of profit using Dynamic Monte Carlo simulations with various stochastic models.
- **Risk Assessment**: Calculates Value at Risk (VaR), Expected Shortfall (ES), and potential profit/loss for simulated price paths.
- **Scoring and Ranking**: Ranks spread opportunities based on a composite score considering multiple factors.

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

### Computational Complexity

STOC'D employs various stochastic models and Monte Carlo simulations to estimate option prices, probabilities, and risks. The computational complexity of these models depends on the number of simulations, the number of time steps, and the complexity of the underlying stochastic processes.

A rough estimate of the computational complexity can be described as:

`O(numExpirations * (avgStrikesPerExpiration)^2 * numProbabilisticModels * numVolatilityEstimates * numMaxSimulations * timeSteps)`

where `timeSteps` is typically 252 (number of trading days in a year) or higher for more granular simulations.

As you can see, the complexity grows exponentially with the number of expirations and strikes, as well as the number of models and volatility estimates used. This can lead to significant computational overhead, especially when running multiple simulations or analyzing a large number of options.

STOC'D Implements robust parallel algorithms to optimize performance and speed up computations, leveraging the power of modern multi-core processors to handle complex calculations efficiently. While this helps reduce the time required for simulations, it requires significant computational resources to run efficiently.

To make STOC'D as resource efficient as possible, the following has been implemented:

- **Parallel Processing**: Utilizes goroutines and channels to run simulations concurrently, improving performance by leveraging multiple CPU cores.
- **Batch Processing**: Processes data in batches to optimize memory usage and reduce overhead, ensuring efficient handling of large datasets.
- **Caching**: Stores intermediate results to avoid redundant calculations and speed up subsequent simulations, enhancing overall performance.
- **Early Simulation Cut-Off**: Implements early stopping criteria to halt simulations that are unlikely to converge, have extremely low probabilities, or do not meet other required criteria (i.e. RoR) saving computational resources and time.

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

### Probabilistic Models

1. **Merton Jump Diffusion Model**: Incorporates jumps in the asset price process.

2. **Kou Jump Diffusion Model**: Uses a double exponential distribution for jump sizes.

3. **CGMY (Carr-Geman-Madan-Yor) Model**: Implements a tempered stable process allowing for infinite activity of small jumps and finite activity of large jumps.

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
- Incorporates multiple volatility estimates and stochastic volatility movement in simulations

### Risk Assessment

- Calculates Value at Risk (VaR) at 95% and 99% confidence levels
- Computes potential profit/loss for simulated price paths
- Calculates Expected Shortfall (ES)
- Calculates Bid-Ask Spread and trading volume for risk assessment

### Scoring and Ranking

- Implements a composite scoring system considering probability of profit, VaR, ES, Bid-Ask Spread, and trading volume
- Normalizes individual factors to create a balanced score
- Weights factors in priority order: liquidity (0.5), probability (0.3), VaR (0.1), ES (0.1)
- Ranks spread opportunities based on the composite score

## Future Enhancements

### Integrate Advanced Volatility Modeling

- **SABR Model Implementation**: Stochastic Alpha Beta Rho (SABR) model for volatility modeling, offering a more flexible framework for capturing the dynamics of implied volatility surfaces.
- **Bates Model Implementation**: Integrate the Bates model, which combines stochastic volatility and jumps, to capture the complex dynamics of asset prices and improve option pricing accuracy.

### Expand One-Dimensional Stochastic Models

- **Longstaff-Schwartz Method**: Implement the Longstaff-Schwartz method for American option pricing, allowing for more accurate pricing of options with early exercise features.
- **Variance Gamma Model**: Implement the Variance Gamma model for more accurate pricing of options, particularly in markets with heavy tails or skewed distributions.
- **Normal Inverse Gaussian (NIG) Model**: Integrate the NIG model to account for skewness and kurtosis in asset returns, offering a more nuanced approach to option pricing and risk management.
- **Enhanced Model Calibrations**: Improve the calibration of existing models, ensuring that they accurately reflect market conditions and historical data, enhancing the reliability of simulations and pricing.

### Develop Multi-Dimensional Stochastic Modeling

- **Lévy Copulas for Multi-Asset Correlation Modeling**: Implement Lévy copulas to model the dependency structures between multiple assets, allowing for more sophisticated multi-asset portfolio simulations and risk assessments.

- **Extended Monte Carlo for Multiple Assets**:
  - **Correlated Asset Price Simulations**: Develop algorithms for simulating correlated price paths across multiple assets, providing insights into the joint behavior of assets in a portfolio.
  - **Multi-Asset Path Generation Algorithms**: Create advanced path generation techniques for scenarios involving multiple assets, enhancing the simulation of complex trading strategies.

- **Multi-Asset Option Pricing Models**:
  - **Basket Option Pricing**: Implement models to price basket options, where the payoff depends on the performance of a group of assets, expanding the tool's capabilities to handle more complex options strategies.

### Improve Greeks Calculations

- **Vomma and Vanna Calculations**: Add support for calculating higher-order Greeks like vomma (sensitivity of vega to volatility) and vanna (sensitivity of delta to volatility), providing deeper insights into options risk management.

- **Numerical Methods for Higher-Order Greeks**: Implement robust numerical techniques for accurately calculating higher-order Greeks, essential for sophisticated hedging and risk management strategies.

### Implement Advanced Hedging Strategies

- **Options Greeks Hedging**:
  - **Delta-Gamma Hedging Algorithms**: Develop algorithms to hedge both delta and gamma, ensuring that portfolios are protected against small and large movements in the underlying asset.
  - **Vega Hedging Strategies**: Create strategies to hedge against volatility risk, ensuring that portfolios are less sensitive to changes in implied volatility.

- **Mean-Variance Hedging**:
  - **Quadratic Hedging Techniques**: Implement mean-variance hedging approaches to minimize the variance of portfolio returns, improving risk-adjusted performance.
  - **Hedging Performance Metrics**: Develop metrics to evaluate the effectiveness of hedging strategies, providing traders with actionable insights into their hedging performance.

- **Dynamic Hedging Strategies**:
  - **Adaptive Hedging Based on Market Conditions**: Implement dynamic hedging strategies that adjust based on real-time market conditions, improving the resilience and adaptability of trading strategies.

### Improve Spread Identification and Analysis

- **More Spread Strategies**:
  - **Iron Condor Identification Algorithm**: Introduce algorithms to identify iron condor opportunities, expanding the range of spread strategies available to traders.
  - **Butterfly Spread Analysis**: Develop tools to analyze butterfly spreads, providing detailed insights into their potential risks and rewards.

- **Enhanced Spread Scoring System**:
  - **Dynamic Weighting Based on Market Conditions**: Refine the spread scoring system to dynamically adjust weightings based on current market conditions, ensuring that the most relevant factors are emphasized in scoring.

### Develop Comprehensive Portfolio Management

- **Position Management**:
  - **Add/Close Position Functions with P&L Tracking**: Implement functionalities for managing open positions, including tools to track profit and loss (P&L) in real-time.
  - **Position Sizing Algorithms**: Develop algorithms for optimal position sizing, helping traders manage risk and maximize returns.

- **Historical Tracking and Analysis**:
  - **Time Series Analysis of Portfolio Performance**: Implement tools for analyzing portfolio performance over time, using time series analysis to identify trends and make data-driven adjustments.

- **Portfolio-Level Tools**:
  - **Portfolio VaR and ES Calculations**: Extend VaR and ES calculations to the portfolio level, providing a holistic view of risk across all positions.
  - **Portfolio Optimization Algorithms**: Develop algorithms to optimize portfolios based on multiple criteria, including risk, return, and capital allocation.
