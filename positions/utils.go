package positions

import (
	"fmt"
	"math"
	"time"

	"github.com/bcdannyboy/dquant/tradier"
)

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
