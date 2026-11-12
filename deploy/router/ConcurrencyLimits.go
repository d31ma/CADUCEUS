package main

type ConcurrencyLimits struct {
	Gates map[string]*FunctionGate
}

func (limits ConcurrencyLimits) Of(functions []string, writers []string, prefix string, others int) ConcurrencyLimits {
	gates := map[string]*FunctionGate{}
	limits.add(gates, functions, prefix, others)
	limits.add(gates, writers, prefix, 1)
	return ConcurrencyLimits{Gates: gates}
}

func (limits ConcurrencyLimits) Gate(function string) *FunctionGate {
	return limits.Gates[function]
}

func (ConcurrencyLimits) add(gates map[string]*FunctionGate, services []string, prefix string, capacity int) {
	for _, service := range services {
		gates[prefix+service] = FunctionGate{}.New(capacity)
	}
}
