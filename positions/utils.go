package positions

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/bcdannyboy/dquant/models"
	"github.com/bcdannyboy/dquant/tradier"
	"github.com/shirou/gopsutil/cpu"
)

func monitorCPUUsage() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var cpuUsage float64
		percentage, err := cpu.Percent(time.Second, false)
		if err == nil && len(percentage) > 0 {
			cpuUsage = percentage[0]
		}
		log.Printf("CPU Usage: %.2f%%", cpuUsage)
	}
}

func estimateTotalJobs(chain map[string]*tradier.OptionChain) int64 {
	total := int64(0)
	for _, expiration := range chain {
		n := int64(len(expiration.Options.Option))
		total += (n * (n - 1)) / 2
	}
	return total
}

func calculateSingleOptionIntrinsicValue(option tradier.Option, underlyingPrice float64) float64 {
	if option.OptionType == "call" {
		return math.Max(0, underlyingPrice-option.Strike)
	}
	return math.Max(0, option.Strike-underlyingPrice)
}

func calculateIntrinsicValue(shortLeg, longLeg models.SpreadLeg, underlyingPrice float64, spreadType string) float64 {
	if spreadType == "Bull Put" {
		return math.Max(0, shortLeg.Option.Strike-longLeg.Option.Strike-(underlyingPrice-longLeg.Option.Strike))
	} else { // Bear Call
		return math.Max(0, longLeg.Option.Strike-shortLeg.Option.Strike-(longLeg.Option.Strike-underlyingPrice))
	}
}

func calculateTimeToMaturity(expirationDate string) float64 {
	expDate, _ := time.Parse("2006-01-02", expirationDate)
	now := time.Now()
	return expDate.Sub(now).Hours() / 24 / 365 // Convert to years
}

func countTotalTasks(chain map[string]*tradier.OptionChain, optionType string) int {
	total := 0
	for _, expiration := range chain {
		var options []tradier.Option
		if optionType == "put" {
			options = filterPutOptions(expiration.Options.Option)
		} else {
			options = filterCallOptions(expiration.Options.Option)
		}
		n := len(options)
		total += (n * (n - 1)) / 2
	}
	return total
}

func printProgress(spreadType string, progress <-chan int, total int) {
	completed := 0
	start := time.Now()

	for range progress {
		completed++
		percentage := float64(completed) / float64(total) * 100
		elapsed := time.Since(start)
		estimatedTotal := elapsed / time.Duration(completed) * time.Duration(total)
		remaining := estimatedTotal - elapsed

		fmt.Printf("\r%s Progress: %.2f%% | Elapsed: %v | Remaining: %v",
			spreadType, percentage, elapsed.Round(time.Second), remaining.Round(time.Second))
	}
	fmt.Println() // Print a newline when done
}

func sanitizeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}
