package controller

import (
	"strings"
)

func CompareProductResult(a, b ProductResponse) int {
	if cmp := strings.Compare(strings.ToLower(a.ProductName), strings.ToLower(b.ProductName)); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.EId, b.EId)
}
