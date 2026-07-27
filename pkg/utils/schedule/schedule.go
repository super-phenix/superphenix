package schedule

import "math/rand/v2"

// RandomHourInRange returns a random hour between minHour and maxHour (inclusive),
// handling the midnight-crossing case where minHour > maxHour (e.g., min=22, max=2
// yields a value in {22, 23, 0, 1, 2}).
func RandomHourInRange(minHour, maxHour int) int {
	// Used to handle negative value in config
	minHour = ((minHour % 24) + 24) % 24
	maxHour = ((maxHour % 24) + 24) % 24

	if minHour == maxHour {
		return minHour
	}

	var rangeSize int
	if maxHour > minHour {
		rangeSize = maxHour - minHour + 1
	} else {
		// wraps around midnight
		rangeSize = (24 - minHour) + maxHour + 1
	}

	return (minHour + rand.IntN(rangeSize)) % 24
}
