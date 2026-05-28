package llm

// FallbackChain stores ordered provider candidates for a tier.
type FallbackChain struct {
	Tier      string
	Providers []Provider
}

func newFallbackChain(tier string, providers []Provider) FallbackChain {
	cp := make([]Provider, 0, len(providers))
	cp = append(cp, providers...)
	return FallbackChain{Tier: tier, Providers: cp}
}
