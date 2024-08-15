package models

import (
	"math"
)

// BesselJ approximates the Bessel function of the first kind
func BesselJ(nu, x float64) float64 {
	if nu < 0 {
		return math.Pow(-1, math.Floor(nu)) * BesselJ(-nu, x)
	}

	if x == 0 {
		if nu == 0 {
			return 1
		}
		return 0
	}

	if math.Abs(x) > 1e-30 {
		return besselJSeries(nu, x)
	}

	return 0
}

func besselJSeries(nu, x float64) float64 {
	const maxIterations = 1000
	const epsilon = 1e-15

	sum := 0.0
	term := math.Pow(x/2, nu) / gamma(nu+1)

	for k := 0; k < maxIterations; k++ {
		sum += term

		term *= -0.25 * x * x / (float64(k+1) * (nu + float64(k+1)))

		if math.Abs(term) < epsilon*math.Abs(sum) {
			break
		}
	}

	return sum
}

// BesselI approximates the modified Bessel function of the first kind
func BesselI(nu, x float64) float64 {
	const maxIterations = 1000
	const epsilon = 1e-15

	sum := 0.0
	term := 1.0 / gamma(nu+1) * math.Pow(x/2, nu)

	for k := 0; k < maxIterations; k++ {
		sum += term

		term *= 0.25 * x * x / (float64(k+1) * (nu + float64(k+1)))

		if math.Abs(term) < epsilon*math.Abs(sum) {
			break
		}
	}

	return sum
}

// BesselK approximates the modified Bessel function of the second kind
func BesselK(nu, x float64) float64 {
	if nu == 0 {
		return besselK0(x)
	}
	if nu == 1 {
		return besselK1(x)
	}

	// Use recurrence relation for nu > 1
	twoNu := 2 * nu
	bkm := besselK0(x)
	bk := besselK1(x)

	for n := 1.0; n < nu; n++ {
		bkp := twoNu/x*bk + bkm
		bkm = bk
		bk = bkp
	}

	return bk
}

func besselK0(x float64) float64 {
	if x <= 2 {
		y := x * x / 4
		return (-math.Log(x/2)*BesselI(0, x) +
			(-0.57721566 + y*(0.42278420+
				y*(0.23069756+y*(0.3488590e-1+
					y*(0.262698e-2+y*(0.10750e-3+
						y*0.74e-5)))))))
	}

	return (math.Exp(-x) / math.Sqrt(x)) *
		(1.25331414 + x*(-0.7832358e-1+
			x*(0.2189568e-1+x*(-0.1062446e-1+
				x*(0.587872e-2+x*(-0.251540e-2+
					x*0.53208e-3))))))
}

func besselK1(x float64) float64 {
	if x <= 2 {
		y := x * x / 4
		return (math.Log(x/2)*BesselI(1, x) +
			(1/x)*(1+y*(0.15443144+
				y*(-0.67278579+y*(-0.18156897+
					y*(-0.1919402e-1+y*(-0.110404e-2+
						y*(-0.4686e-4))))))))
	}

	return (math.Exp(-x) / math.Sqrt(x)) *
		(1.25331414 + x*(0.23498619+
			x*(-0.3655620e-1+x*(0.1504268e-1+
				x*(-0.780353e-2+x*(0.325614e-2+
					x*(-0.68245e-3)))))))
}

// gamma approximates the gamma function
func gamma(x float64) float64 {
	if x <= 0 {
		return math.NaN()
	}

	if x < 1 {
		return gamma(x+1) / x
	}

	if x > 171 {
		return math.Inf(1)
	}

	return math.Gamma(x)
}
