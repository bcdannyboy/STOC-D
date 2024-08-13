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
   - [Stochastic Models](#stochastic-models)
   - [Monte Carlo Simulation](#monte-carlo-simulation)
6. [Roadmap](#roadmap)

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

1. **Yang-Zhang Volatility**: This method provides a more accurate estimation of volatility by considering opening, closing, high, and low prices. It's particularly useful for assets with significant overnight price jumps.

   Implementation: `models/yang.go`

   The Yang-Zhang estimator is calculated as:

   ```
   σ_YZ^2 = σ_O^2 + k * σ_C^2 + (1 - k) * σ_RS^2
   ```

   Where:
   - σ_O^2 is the opening price volatility
   - σ_C^2 is the closing price volatility
   - σ_RS^2 is the Rogers-Satchell volatility
   - k is a weighting factor

2. **Rogers-Satchell Volatility**: This estimator is drift-independent and uses high, low, opening, and closing prices.

   Implementation: `models/rogers.go`

   The Rogers-Satchell volatility is calculated as:

   ```
   σ_RS^2 = ln(H/C) * ln(H/O) + ln(L/C) * ln(L/O)
   ```

   Where H, L, O, C are the high, low, opening, and closing prices respectively.

3. **Local Volatility Surface**: This method creates a volatility surface based on option prices across different strikes and expirations.

   Implementation: `models/local_vol.go`

   The local volatility is interpolated using:

   ```
   σ_local(K, T) = Interpolate(K, T, σ_implied)
   ```

   Where K is the strike price, T is the time to expiration, and σ_implied is the implied volatility from market prices.

### Stochastic Models

STOC'D utilizes three main stochastic models for price simulation:

1. **Heston Stochastic Volatility Model**: This model allows for mean-reverting stochastic volatility, which can capture volatility clustering and leverage effects.

   Implementation: `models/heston.go`

   The Heston model is defined by the following stochastic differential equations:

   ```
   dS(t) = μS(t)dt + √v(t)S(t)dW_1(t)
   dv(t) = κ(θ - v(t))dt + ξ√v(t)dW_2(t)
   ```

   Where:
   - S(t) is the asset price
   - v(t) is the variance
   - μ is the drift
   - κ is the rate of mean reversion
   - θ is the long-term variance
   - ξ is the volatility of volatility
   - W_1 and W_2 are Wiener processes with correlation ρ

2. **Merton Jump Diffusion Model**: This model incorporates jumps in the asset price, allowing for sudden, significant price movements.

   Implementation: `models/merton.go`

   The Merton jump diffusion model is defined as:

   ```
   dS(t) = (μ - λk)S(t)dt + σS(t)dW(t) + J(t)S(t)dN(t)
   ```

   Where:
   - λ is the average number of jumps per unit time
   - k is the average jump size
   - J(t) is the jump size (typically log-normally distributed)
   - N(t) is a Poisson process

3. **Kou Jump Diffusion Model**: Similar to the Merton model, but with a double exponential distribution for jump sizes.

   Implementation: `models/kuo.go`

   The Kou model is defined similarly to the Merton model, but with a different jump size distribution:

   ```
   J(t) = exp(Y) - 1
   ```

   Where Y follows a double exponential distribution.

### Monte Carlo Simulation

STOC'D uses Monte Carlo simulation to estimate the probability of profit for identified credit spreads. This involves simulating thousands of price paths using the stochastic models and calculating the proportion of paths that result in a profitable outcome.

Implementation: `probability/ivmc.go`

The simulation process:

1. Generate multiple price paths using the chosen stochastic model
2. For each path, determine if the spread would be profitable at expiration
3. Calculate the proportion of profitable paths to estimate the probability of profit

## Roadmap

- [x] Implement volatility estimation techniques
  - [x] Yang-Zhang Volatility
  - [x] Rogers-Satchell Volatility
  - [x] Local Volatility Surface
  - [x] Heston Stochastic Volatility Model
- [x] Implement one-dimensional stochastic models with jumps for price simulation
  - [x] Merton Jump Diffusion Model
  - [x] Kuo Jump Diffusion Model
  - [ ] Variance-Gamma
  - [ ] Normal-Inverse Gaussian
  - [ ] Generalized-Hyperbolic Model
  - [ ] CGMY Tempered Stable Process Model
  - [ ] Generalized Hyperbolic Model
- [ ] Implement multi-dimensional stochastic models with jumps for price simulation and dependence modelling
  - [ ] Levy Copulas for dependence modelling
- [ ] Hedging Mechanisms
  - [ ] Superhedging
  - [ ] Options Greeks Hedging
  - [ ] Mean-Variance Heding
- [ ] Add portfolio management
  - [ ] Position add / close capabilities
  - [ ] Historical position tracking
  - [ ] Profit / loss tracking
