package schedule

import "testing"

func TestRandomHourInRange(t *testing.T) {
	tests := []struct {
		name     string
		min      int
		max      int
		validFn  func(int) bool
		describe string
	}{
		{
			name:     "normal range 0-20",
			min:      0,
			max:      20,
			validFn:  func(h int) bool { return h >= 0 && h <= 20 },
			describe: "hour in [0,20]",
		},
		{
			name: "wrapping range 22-2",
			min:  22,
			max:  2,
			validFn: func(h int) bool {
				return h == 22 || h == 23 || h == 0 || h == 1 || h == 2
			},
			describe: "hour in {22,23,0,1,2}",
		},
		{
			name:     "equal min and max",
			min:      5,
			max:      5,
			validFn:  func(h int) bool { return h == 5 },
			describe: "hour == 5",
		},
		{
			name:     "single hour range 3-4",
			min:      3,
			max:      4,
			validFn:  func(h int) bool { return h == 3 || h == 4 },
			describe: "hour in {3,4}",
		},
		{
			name:     "full day 0-23",
			min:      0,
			max:      23,
			validFn:  func(h int) bool { return h >= 0 && h <= 23 },
			describe: "hour in [0,23]",
		},
		{
			name:     "wrapping edge 23-0",
			min:      23,
			max:      0,
			validFn:  func(h int) bool { return h == 23 || h == 0 },
			describe: "hour in {23,0}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 1000; i++ {
				h := RandomHourInRange(tt.min, tt.max)
				if !tt.validFn(h) {
					t.Fatalf("expected %s, got %d", tt.describe, h)
				}
			}
		})
	}
}
