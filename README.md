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

1. **Yang-Zhang Volatility**: This method provides a more accurate estimation of volatility by considering opening, closing, high, and low prices. It's particularly useful for assets with significant overnight price jumps.

   The Yang-Zhang estimator is calculated as:

   ```
   σ_YZ^2 = σ_O^2 + k * σ_C^2 + (1 - k) * σ_RS^2
   ```

   Where σ_O^2 is the opening price volatility, σ_C^2 is the closing price volatility, σ_RS^2 is the Rogers-Satchell volatility, and k is a weighting factor.

2. **Rogers-Satchell Volatility**: This estimator is drift-independent and uses high, low, opening, and closing prices.

   The Rogers-Satchell volatility is calculated as:

   ```
   σ_RS^2 = ln(H/C) * ln(H/O) + ln(L/C) * ln(L/O)
   ```

   Where H, L, O, C are the high, low, opening, and closing prices respectively.

3. **Local Volatility Surface**: This method creates a volatility surface based on option prices across different strikes and expirations.

   The local volatility is interpolated using:

   ```
   σ_local(K, T) = Interpolate(K, T, σ_implied)
   ```

   Where K is the strike price, T is the time to expiration, and σ_implied is the implied volatility from market prices.

### One-Dimensional Stochastic Models

STOC'D utilizes the following one-dimensional stochastic models for price simulation:

1. **Heston Stochastic Volatility Model**: This model allows for mean-reverting stochastic volatility.

   The Heston model is defined by the following stochastic differential equations:

   ```
   dS(t) = μS(t)dt + √v(t)S(t)dW_1(t)
   dv(t) = κ(θ - v(t))dt + ξ√v(t)dW_2(t)
   ```

   Where S(t) is the asset price, v(t) is the variance, μ is the drift, κ is the rate of mean reversion, θ is the long-term variance, ξ is the volatility of volatility, and W_1 and W_2 are Wiener processes with correlation ρ.

2. **Merton Jump Diffusion Model**: This model incorporates jumps in the asset price.

   The Merton jump diffusion model is defined as:

   ```
   dS(t) = (μ - λk)S(t)dt + σS(t)dW(t) + J(t)S(t)dN(t)
   ```

   Where λ is the average number of jumps per unit time, k is the average jump size, J(t) is the jump size (typically log-normally distributed), and N(t) is a Poisson process.

3. **Kou Jump Diffusion Model**: Similar to the Merton model, but with a double exponential distribution for jump sizes.

   The Kou model is defined similarly to the Merton model, but with a different jump size distribution:

   ```
   J(t) = exp(Y) - 1
   ```

   Where Y follows a double exponential distribution.

4. **CGMY Tempered Stable Process Model**: This model allows for infinite activity of small jumps and finite activity of large jumps.

   The CGMY process is defined by its characteristic function:

   ```
   φ(u) = exp(CΓ(-Y)[(M - iu)^Y - M^Y + (G + iu)^Y - G^Y])
   ```

   Where C, G, M, and Y are parameters controlling the process behavior.

### Multi-Dimensional Stochastic Models

STOC'D plans to implement multi-dimensional stochastic models for price simulation and dependence modeling:

1. **Levy Copulas**: These will be used for dependence modeling between multiple assets, allowing for more accurate portfolio simulations.

   A Lévy copula C for a d-dimensional Lévy process X = (X_1, ..., X_d) is defined as:

   ```
   C(u_1, ..., u_d) = U(F_1^{-1}(u_1), ..., F_d^{-1}(u_d))
   ```

   Where U is the tail integral of X and F_i^{-1} are the inverse marginal tail integrals.

### Hedging Mechanisms

STOC'D aims to implement various hedging mechanisms:

1. **Options Greeks Hedging**: This involves using the Greeks (delta, gamma, vega, theta) to create a hedged portfolio.

   For example, delta-hedging involves maintaining a position of -Δ in the underlying for each long option, where Δ is the first derivative of the option price with respect to the underlying price.

2. **Mean-Variance Hedging**: This approach aims to find the self-financing strategy that minimizes the expected squared hedging error.

   The optimal strategy ξ* is the solution to:

   ```
   min_ξ E[(H - (x + ∫_0^T ξ_t dS_t))^2]
   ```

   Where H is the payoff to be hedged, x is the initial capital, and S is the price process.

### Monte Carlo Simulation

STOC'D uses Monte Carlo simulation to estimate the probability of profit for identified credit spreads. This involves simulating thousands of price paths using the stochastic models and calculating the proportion of paths that result in a profitable outcome.

## Portfolio Management

STOC'D plans to implement comprehensive portfolio management features:

1. **Position Add / Close Capabilities**: Ability to add new positions or close existing ones, including calculation of P&L and impact on overall portfolio risk metrics.

2. **Historical Position Tracking**: Keep track of all historical positions for analysis and reporting, including time series of Greeks and risk exposures.

3. **Profit / Loss Tracking**: Real-time and historical profit/loss tracking for individual positions and the overall portfolio, including unrealized and realized P&L, and performance attribution.

## Roadmap

- [x] Implement volatility estimation techniques
  - [x] Yang-Zhang Volatility
  - [x] Rogers-Satchell Volatility
  - [x] Local Volatility Surface
  - [x] Heston Stochastic Volatility Model
- [x] Implement one-dimensional stochastic models with jumps for price simulation
  - [x] Merton Jump Diffusion Model
  - [x] Kou Jump Diffusion Model
  - [x] CGMY Tempered Stable Process Model
- [ ] Implement multi-dimensional stochastic models with jumps for price simulation and dependence modelling
  - [ ] Levy Copulas for dependence modelling
- [ ] Hedging Mechanisms
  - [ ] Options Greeks Hedging
  - [ ] Mean-Variance Hedging
- [ ] Add portfolio management
  - [ ] Position add / close capabilities
  - [ ] Historical position tracking
  - [ ] Profit / loss tracking