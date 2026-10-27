package dbaas

import "regexp"

var (
	CpuValueList    = []int{1, 2, 4, 8, 16}
	MemoryValueList = []int{1, 2, 4, 8, 16, 32, 64}

	// vipRegex matches the 198.18.0.0/16 load balancer range (SPX RFC008), like the sfs-dbaas chart.
	vipRegex = regexp.MustCompile(`^198\.18\.(25[0-5]|2[0-4][0-9]|1?[0-9]{1,2})\.(25[0-5]|2[0-4][0-9]|1?[0-9]{1,2})$`)
)

const (
	AppPrefix = "dbaas"
	// eipLocalIDPrefix prefixes the database localID in the localID of its EIP, like the sfs-dbaas chart.
	eipLocalIDPrefix = "eip."

	quantityFormat = "%dGi"

	minInstances = 1
	maxInstances = 5
	minStorage   = 1
)
