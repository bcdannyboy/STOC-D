# STOC'D: Stochastic Optimization for Credit Spread Decision Making

![STOC'D Logo](./stocd.webp)

STOC'D (Stochastic Optimization for Credit Spread Decision Making) is an advanced options trading analysis tool that employs various stochastic models and volatility estimation techniques to identify optimal credit spread opportunities.

## Table of Contents

1. [Introduction](#introduction)
2. [Features](#features)
3. [Installation](#installation)
4. [Usage](#usage)
5. [Technical Details](#technical-details)
   - [Volatility Estimation](#volatility-estimation)
   - [One-Dimensional Stochastic Models](#one-dimensional-stochastic-models)
   - [Multi-Dimensional Stochastic Models](#multi-dimensional-stochastic-models)
   - [Hedging Mechanisms](#hedging-mechanisms)
   - [Monte Carlo Simulation](#monte-carlo-simulation)
6. [Portfolio Management](#portfolio-management)
7. [Roadmap](#roadmap)

## Introduction

STOC'D is designed to assist traders in making informed decisions about credit spread strategies. It combines historical data analysis, options chain information, and advanced stochastic models to provide comprehensive insights into potential trades.

## Features

- Fetch and analyze historical price data and options chains
- Implement multiple volatility estimation techniques
- Utilize advanced stochastic models with jumps for price simulation
- Identify optimal credit spread opportunities
- Perform Monte Carlo simulations for probability estimation

## Installation

1. Clone the repository:

   ```
   git clone https://github.com/bcdannyboy/stocd.git
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

### Volatility Estimation

STOC'D implements several volatility estimation techniques:

1. **Yang-Zhang Volatility**: This method provides a more accurate estimation of volatility by considering opening, closing, high, and low prices.

2. **Rogers-Satchell Volatility**: This estimator is drift-independent and uses high, low, opening, and closing prices.

3. **Local Volatility Surface**: This method creates a volatility surface based on option prices across different strikes and expirations.

### One-Dimensional Stochastic Models

STOC'D utilizes the following one-dimensional stochastic models for price simulation:

1. **Heston Stochastic Volatility Model**: This model allows for mean-reverting stochastic volatility.

2. **Merton Jump Diffusion Model**: This model incorporates jumps in the asset price.

3. **Kou Jump Diffusion Model**: Similar to the Merton model, but with a double exponential distribution for jump sizes.

4. **Variance-Gamma Model**: This model uses a gamma process to time-change Brownian motion, allowing for higher kurtosis and skewness.

5. **Normal-Inverse Gaussian Model**: This model is based on the normal-inverse Gaussian distribution and can capture both skewness and kurtosis in returns.

6. **Generalized Hyperbolic Model**: This model provides a flexible class of distributions that includes many other models as special cases.

7. **CGMY Tempered Stable Process Model**: This model allows for infinite activity of small jumps and finite activity of large jumps.

### Multi-Dimensional Stochastic Models

STOC'D plans to implement multi-dimensional stochastic models for price simulation and dependence modeling:

1. **Levy Copulas**: These will be used for dependence modeling between multiple assets, allowing for more accurate portfolio simulations.

### Hedging Mechanisms

STOC'D aims to implement various hedging mechanisms:

1. **Superhedging**: This technique aims to find the cheapest portfolio that dominates the payoff of a given contingent claim.

2. **Options Greeks Hedging**: This involves using the Greeks (delta, gamma, vega, theta) to create a hedged portfolio.

3. **Mean-Variance Hedging**: This approach aims to find the self-financing strategy that minimizes the expected squared hedging error.

### Monte Carlo Simulation

STOC'D uses Monte Carlo simulation to estimate the probability of profit for identified credit spreads.

## Portfolio Management

STOC'D plans to implement comprehensive portfolio management features:

1. **Position Add / Close Capabilities**: Ability to add new positions or close existing ones.

2. **Historical Position Tracking**: Keep track of all historical positions for analysis and reporting.

3. **Profit / Loss Tracking**: Real-time and historical profit/loss tracking for individual positions and the overall portfolio.

## Roadmap

- [x] Implement volatility estimation techniques
  - [x] Yang-Zhang Volatility
  - [x] Rogers-Satchell Volatility
  - [x] Local Volatility Surface
  - [x] Heston Stochastic Volatility Model
- [x] Implement one-dimensional stochastic models with jumps for price simulation
  - [x] Merton Jump Diffusion Model
  - [x] Kou Jump Diffusion Model
  - [ ] Variance-Gamma Model
  - [ ] Normal-Inverse Gaussian Model
  - [ ] Generalized Hyperbolic Model
  - [ ] CGMY Tempered Stable Process Model
- [ ] Implement multi-dimensional stochastic models with jumps for price simulation and dependence modelling
  - [ ] Levy Copulas for dependence modelling
- [ ] Hedging Mechanisms
  - [ ] Superhedging
  - [ ] Options Greeks Hedging
  - [ ] Mean-Variance Hedging
- [ ] Add portfolio management
  - [ ] Position add / close capabilities
  - [ ] Historical position tracking
  - [ ] Profit / loss tracking