package models

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/optimize"
)

type VarianceGamma struct {
	Mu     float64 // Location parameter: Shifts the distribution left or right
	Alpha  float64 // Shape parameter: Controls the overall shape of the distribution
	Beta   float64 // Skewness parameter: Controls the asymmetry of the distribution
	Lambda float64 // Rate parameter: Inversely related to the kurtosis (tail thickness) of the distribution
}

func (vg *VarianceGamma) PDF(x float64) float64 {
	gamma := math.Sqrt(math.Max(0, vg.Alpha*vg.Alpha-vg.Beta*vg.Beta))
	if gamma == 0 {
		return 0
	}

	term1 := math.Pow(gamma, 2*vg.Lambda) * math.Pow(math.Abs(x-vg.Mu), vg.Lambda-0.5)
	term2 := BesselK(vg.Lambda-0.5, vg.Alpha*math.Abs(x-vg.Mu))
	term3 := math.Sqrt(math.Pi) * math.Gamma(vg.Lambda) * math.Pow(2*vg.Alpha, vg.Lambda-0.5)
	term4 := math.Exp(vg.Beta * (x - vg.Mu))

	if term3 == 0 {
		return 0
	}

	result := (term1 * term2 * term4) / term3
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}

	return result
}

func (vg *VarianceGamma) Fit(data []float64) {
	fmt.Println("Starting Variance-Gamma model fitting...")
	fmt.Printf("Input data size: %d\n", len(data))

	min, max, mean, std := minMax(data)
	fmt.Printf("Data statistics: min=%f, max=%f, mean=%f, std=%f\n", min, max, mean, std)

	// Improved initial guess
	initialGuess := []float64{
		mean,              // Mu
		std,               // Alpha
		0.1 * std,         // Beta (small initial skew)
		1.0 / (std * std), // Lambda
	}

	problem := optimize.Problem{
		Func: func(params []float64) float64 {
			vg.Mu = params[0]
			vg.Alpha = math.Abs(params[1]) // Ensure Alpha is positive
			vg.Beta = params[2]
			vg.Lambda = math.Abs(params[3]) // Ensure Lambda is positive

			logLikelihood := 0.0
			for _, x := range data {
				ll := math.Log(vg.PDF(x))
				if math.IsInf(ll, 0) || math.IsNaN(ll) {
					return math.Inf(1)
				}
				logLikelihood += ll
			}
			return -logLikelihood
		},
	}

	result, err := optimize.Minimize(problem, initialGuess, nil, &optimize.NelderMead{})
	if err != nil {
		fmt.Printf("Error in optimization: %v\n", err)
		return
	}

	vg.Mu = result.X[0]
	vg.Alpha = math.Abs(result.X[1])
	vg.Beta = result.X[2]
	vg.Lambda = math.Abs(result.X[3])

	fmt.Printf("Finished fitting. Final parameters: Mu=%f, Alpha=%f, Beta=%f, Lambda=%f\n",
		vg.Mu, vg.Alpha, vg.Beta, vg.Lambda)
}
func (vg *VarianceGamma) Mean() float64 {
	gamma := math.Sqrt(vg.Alpha*vg.Alpha - vg.Beta*vg.Beta)
	return vg.Mu + (2*vg.Beta*vg.Lambda)/(gamma*gamma)
}

func (vg *VarianceGamma) Variance() float64 {
	gamma := math.Sqrt(vg.Alpha*vg.Alpha - vg.Beta*vg.Beta)
	return (2 * vg.Lambda * (1 + 2*vg.Beta*vg.Beta/(gamma*gamma))) / (gamma * gamma)
}

func (vg *VarianceGamma) Skewness() float64 {
	gamma := math.Sqrt(vg.Alpha*vg.Alpha - vg.Beta*vg.Beta)
	numerator := 2 * vg.Beta * (3*gamma*gamma + 2*vg.Beta*vg.Beta) / math.Pow(gamma, 3)
	denominator := math.Pow(1+2*vg.Beta*vg.Beta/(gamma*gamma), 1.5)
	return numerator / (math.Sqrt(vg.Lambda) * denominator)
}

func (vg *VarianceGamma) Kurtosis() float64 {
	gamma := math.Sqrt(vg.Alpha*vg.Alpha - vg.Beta*vg.Beta)
	term1 := 3 * (gamma*gamma*gamma*gamma + 4*gamma*gamma*vg.Beta*vg.Beta + 2*vg.Beta*vg.Beta*vg.Beta*vg.Beta)
	term2 := math.Pow(gamma, 4) * (1 + 2*vg.Beta*vg.Beta/(gamma*gamma))
	return 3 + term1/(vg.Lambda*term2)
}

func minMax(data []float64) (min, max, mean, std float64) {
	if len(data) == 0 {
		return
	}
	min, max = data[0], data[0]
	sum := 0.0
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	mean = sum / float64(len(data))
	sumSq := 0.0
	for _, v := range data {
		sumSq += (v - mean) * (v - mean)
	}
	std = math.Sqrt(sumSq / float64(len(data)))
	return
}
