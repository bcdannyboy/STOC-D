package models

import (
	"math"
	"sort"
	"time"

	"github.com/bcdannyboy/stocd/tradier"
	"golang.org/x/exp/rand"
)

type VolatilitySurface struct {
	Strikes []float64
	Times   []float64
	Vols    [][]float64
}

func CalculateLocalVolatilitySurface(chain map[string]*tradier.OptionChain, underlyingPrice float64) VolatilitySurface {
	var strikes [][]float64
	var times []float64
	var vols [][]float64

	for expDate, expChain := range chain {
		t, err := time.Parse("2006-01-02", expDate)
		if err != nil {
			continue
		}
		timeToExpiry := t.Sub(time.Now()).Hours() / 24 / 365 // in years

		var strikeVols []struct {
			strike float64
			vol    float64
		}

		for _, opt := range expChain.Options.Option {
			iv := (opt.Greeks.BidIv + opt.Greeks.AskIv) / 2
			if iv > 0 {
				strikeVols = append(strikeVols, struct {
					strike float64
					vol    float64
				}{opt.Strike, iv})
			}
		}

		if len(strikeVols) > 0 {
			sort.Slice(strikeVols, func(i, j int) bool {
				return strikeVols[i].strike < strikeVols[j].strike
			})

			times = append(times, timeToExpiry)
			strikeSlice := make([]float64, 0, len(strikeVols))
			volSlice := make([]float64, 0, len(strikeVols))

			for _, sv := range strikeVols {
				strikeSlice = append(strikeSlice, sv.strike)
				volSlice = append(volSlice, sv.vol)
			}

			strikes = append(strikes, strikeSlice)
			vols = append(vols, volSlice)
		}
	}

	// Ensure all strike slices have the same length
	maxStrikes := 0
	for _, s := range strikes {
		if len(s) > maxStrikes {
			maxStrikes = len(s)
		}
	}

	for i := range strikes {
		for len(strikes[i]) < maxStrikes {
			strikes[i] = append(strikes[i], strikes[i][len(strikes[i])-1])
			vols[i] = append(vols[i], vols[i][len(vols[i])-1])
		}
	}

	// Flatten strikes to a single slice
	flatStrikes := []float64{}
	for _, s := range strikes {
		flatStrikes = append(flatStrikes, s...)
	}
	sort.Float64s(flatStrikes)
	uniqueStrikes := removeDuplicates(flatStrikes)

	return VolatilitySurface{
		Strikes: uniqueStrikes,
		Times:   times,
		Vols:    vols,
	}
}

func removeDuplicates(sorted []float64) []float64 {
	if len(sorted) == 0 {
		return sorted
	}
	result := []float64{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			result = append(result, sorted[i])
		}
	}
	return result
}

func InterpolateVolatility(surface VolatilitySurface, S, t float64) float64 {
	if len(surface.Strikes) == 0 || len(surface.Times) == 0 || len(surface.Vols) == 0 {
		return 0 // Return a default value if the surface is empty
	}

	// Find the time indices
	tIndex := sort.SearchFloat64s(surface.Times, t)
	if tIndex == len(surface.Times) {
		tIndex--
	}

	// Find the strike indices
	sIndex := sort.SearchFloat64s(surface.Strikes, S)
	if sIndex == len(surface.Strikes) {
		sIndex--
	}

	// Ensure we're within bounds
	tIndex = clamp(tIndex, 0, len(surface.Vols)-1)
	sIndex = clamp(sIndex, 0, len(surface.Vols[tIndex])-1)

	// If we're at the edge, return the nearest value
	if tIndex == len(surface.Times)-1 || sIndex == len(surface.Strikes)-1 {
		return surface.Vols[tIndex][sIndex]
	}

	// Perform bilinear interpolation
	t0, t1 := surface.Times[tIndex], surface.Times[tIndex+1]
	s0, s1 := surface.Strikes[sIndex], surface.Strikes[sIndex+1]

	v00 := surface.Vols[tIndex][sIndex]
	v01 := surface.Vols[tIndex][sIndex+1]
	v10 := surface.Vols[tIndex+1][sIndex]
	v11 := surface.Vols[tIndex+1][sIndex+1]

	xt := (t - t0) / (t1 - t0)
	xs := (S - s0) / (s1 - s0)

	return (1-xt)*(1-xs)*v00 + xt*(1-xs)*v10 + (1-xt)*xs*v01 + xt*xs*v11
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func SimulateLocalVolPath(S0, r float64, surface VolatilitySurface, T float64, steps int, rng *rand.Rand) []float64 {
	dt := T / float64(steps)
	sqrtDt := math.Sqrt(dt)

	S := make([]float64, steps+1)
	S[0] = S0

	for i := 0; i < steps; i++ {
		t := float64(i) * dt
		vol := InterpolateVolatility(surface, S[i], t)
		S[i+1] = S[i] * math.Exp((r-0.5*vol*vol)*dt+vol*sqrtDt*rng.NormFloat64())
	}

	return S
}
