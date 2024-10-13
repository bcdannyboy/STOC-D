# STOC'D: Stochastic Trade Optimization for Credit Derivatives

![STOC'D Logo](./stocd.webp)

STOC'D (Stochastic Trade Optimization for Credit Derivatives) is an advanced options trading analysis tool that employs various stochastic models and volatility estimation techniques to identify optimal credit spread opportunities. It is implemented as a Slack app, allowing users to interact with it directly through Slack commands.

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
6. [Slack Integration](#slack-integration)
7. [Additional Commands](#additional-commands)
8. [Future Enhancements](#future-enhancements)

## Introduction

STOC'D is designed to assist traders in making informed decisions about credit spread strategies. It combines historical data analysis, options chain information, and advanced stochastic models to provide comprehensive insights into potential trades. As a Slack app, it allows users to easily access these powerful analytics tools directly from their Slack workspace.

## Features

- **Slack Integration**: Interact with STOC'D directly through Slack commands.
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

3. Set up your environment variables in a `.env` file:

   ```
   TRADIER_KEY=your_tradier_api_key_here
   SLACK_APP_TOKEN=your_slack_app_token_here
   SLACK_BOT_TOKEN=your_slack_bot_token_here
   ```

4. Build the application:

   ```
   go build -o stocd .
   ```

## Usage

Run the STOC'D Slack bot:

```
./stocd
```

Once the bot is running, you can interact with it in your Slack workspace using the following commands:

- `/help`: Display available commands and their usage.
- `/fcs <symbol> <indicator> <minDTE> <maxDTE> <minRoR> <RFR>`: Find credit spreads for a given symbol.

Example:
```
/fcs AAPL 1 14 30 0.175 0.0382
```

This command will analyze Apple (AAPL) options, looking for bull put spreads (indicator > 0) with 14-30 days to expiration, a minimum return on risk of 17.5%, and using a risk-free rate of 3.82%.

## Technical Details

### Computational Complexity

STOC'D employs various stochastic models and Monte Carlo simulations to estimate option prices, probabilities, and risks. The computational complexity depends on the number of simulations, time steps, and the complexity of the underlying stochastic processes.

An estimate of the computational requirements for the `fcs` command can be calculated as such:

`O(numExpirations * (avgStrikesPerExpiration)^2 * numProbabilisticModels * numVolatilityEstimates * numMaxSimulations * timeSteps)`

as you can see, the complexity is exponential in the number of parameters. To mitigate this, STOC'D implements parallel processing, batch processing, caching, and early simulation cut-off to optimize performance and resource usage.

### Data Fetching

- Utilizes the Tradier API to fetch historical price data, options chains, and price statistics.
- Implements functions to retrieve quotes, options expirations, and full options chains.

### Volatility Estimation

1. **Yang-Zhang Volatility**: Accounts for overnight jumps.
2. **Rogers-Satchell Volatility**: Independent of price drift.
3. **Local Volatility Surface**: Based on option prices across different strikes and expirations.
4. **Implied Volatility**: Calculated using the Black-Scholes-Merton model.
5. **Historical Volatility**: Computed from past price data.
6. **Heston Stochastic Volatility**: Uses mean-reverting stochastic volatility.

### Probabilistic Models

1. **Merton Jump Diffusion Model**: Incorporates price jumps.
2. **Kou Jump Diffusion Model**: Uses double exponential distribution for jump sizes.
3. **CGMY Model**: Implements a tempered stable process for jumps.

### Option Pricing

- Implements Black-Scholes-Merton formula for European option pricing.
- Calculates option Greeks (Delta, Gamma, Theta, Vega, Rho).
- Computes implied volatility using numerical methods.

### Spread Identification

- Identifies potential Bull Put and Bear Call spread opportunities.
- Filters spreads based on Days to Expiration (DTE) and return on risk (ROR).

### Probability Calculation

- Performs Monte Carlo simulations using various stochastic models.
- Estimates probability of profit for identified spreads.
- Incorporates multiple volatility estimates and stochastic volatility.

### Risk Assessment

- Calculates Value at Risk (VaR) at 95% and 99% confidence levels.
- Computes Expected Shortfall (ES).
- Assesses risk based on Bid-Ask Spread and trading volume.

### Scoring and Ranking

- Implements a composite scoring system considering probability of profit, VaR, ES, Bid-Ask Spread, and trading volume.
- Normalizes and weights factors to create a balanced score.
- Ranks spread opportunities based on the composite score.

## Slack Integration

STOC'D is implemented as a Slack app, allowing users to interact with it directly through Slack commands. The integration includes:

- Slash commands for triggering analyses and retrieving help information.
- Real-time progress updates during analysis.
- Formatted messages for displaying results and error information.
- Handling of concurrent requests from multiple users.

## Additional Commands

- `/ee` - entry / exit, provide stochastic best entry and exit points for a given symbol.
- `/pv` - predict volatility, provide a prediction of the volatility for a given symbol.
- `/bsi` - buy / sell indicator, provide a buy or sell indicator for a given symbol.
- `/ca` - cointegration analysis, provide a cointegration analysis for a given set of symbols.
- `/ns` - news sentiment, provide a sentiment analysis of the news for a given symbol.
- `/it` - insider trading, provide an insider trading analysis for a given symbol.
- `/ne` - next earnings, provide the date and days until the next earnings report for a given symbol.

## Future Enhancements

- **Advanced Volatility Modeling**
   - Implement SABR and Bates models.

- **Expand One-Dimensional Stochastic Models**
   - Add Longstaff-Schwartz, Variance Gamma, and Normal Inverse Gaussian models.
   - Enhance model calibrations.

- **Develop Multi-Dimensional Stochastic Modeling**
   - Implement Lévy Copulas for multi-asset correlation.
   - Extend Monte Carlo for multiple assets.
   - Develop multi-asset option pricing models.

- **Improve Greeks Calculations**
   - Add support for higher-order Greeks like vomma and vanna.

- **Implement Advanced Hedging Strategies**
   - Develop delta-gamma and vega hedging algorithms.
   - Implement mean-variance and dynamic hedging strategies.

- **Improve Spread Identification and Analysis**
   - Add support for more spread strategies (e.g., iron condors, butterflies).
   - Enhance the spread scoring system with dynamic weighting.

- **Develop Comprehensive Portfolio Management**
   - Implement position management and tracking.
   - Add portfolio-level risk analysis and optimization tools.

- **Enhance Slack Integration**
   - Implement interactive messages for easier navigation of results.
   - Add support for custom alerts and notifications.
   - Develop a dashboard for monitoring multiple analyses simultaneously.

- **Improve Performance and Scalability**
   - Optimize Monte Carlo simulations for better performance.
   - Implement distributed computing for handling larger datasets and more complex analyses.

- **Enhance Data Sources and Analysis**
    - Integrate additional market data providers.
    - Implement sentiment analysis from news and social media sources.