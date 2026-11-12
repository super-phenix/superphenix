package vm

import v1 "kubevirt.io/api/core/v1"

var (
	RUN_STRATEGY_VALUE_LIST = []string{
		string(v1.RunStrategyAlways),
		string(v1.RunStrategyRerunOnFailure),
		string(v1.RunStrategyOnce),
		string(v1.RunStrategyManual),
		string(v1.RunStrategyHalted),
	}
)
