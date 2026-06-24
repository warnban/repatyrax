package service

import "math"

var tierBasePrices = map[string]float64{
	"CORE":     199,
	"SHADOW":   349,
	"DOMINION": 649,
}

var periodDiscounts = map[int]float64{
	1:  1.00,
	3:  0.90,
	6:  0.85,
	12: 0.80,
}

// CalculatePrice returns the total RUB amount for a given tier and billing period.
// Discount is applied to the full period total, then rounded to the nearest ruble.
func CalculatePrice(tier string, months int) float64 {
	base, ok := tierBasePrices[tier]
	if !ok {
		return 0
	}
	discount, ok := periodDiscounts[months]
	if !ok {
		discount = 1.0
	}
	return math.Round(base * float64(months) * discount)
}
