package llm

// FallbackChain stores ordered provider candidates for a level group.
type FallbackChain struct {
	LevelGroup string
	Providers  []Provider
}

func newFallbackChain(levelGroup string, providers []Provider) FallbackChain {
	cp := make([]Provider, 0, len(providers))
	cp = append(cp, providers...)
	return FallbackChain{LevelGroup: levelGroup, Providers: cp}
}
