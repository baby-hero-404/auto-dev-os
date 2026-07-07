package repomap

// CalculatePageRank computes Personalized PageRank manually because gonum's 
// built-in network.PageRank does not support personalization vectors.
// activeFiles receive a heavy initialization boost, pushing rank towards their dependencies.
func (g *DependencyGraph) CalculatePageRank(activeFiles []string) map[string]float64 {
	d := 0.85 // Damping factor
	iterations := 50
	
	nodesMap := g.nodes
	numNodes := len(nodesMap)
	if numNodes == 0 {
		return map[string]float64{}
	}
	
	// 1. Initialize personalization vector (Boost active files)
	pers := make(map[int64]float64)
	if len(activeFiles) > 0 {
		boost := 1.0 / float64(len(activeFiles))
		for _, f := range activeFiles {
			if n, exists := g.nodes[f]; exists {
				pers[n.ID()] = boost
			}
		}
	}
	
	// Fallback: Uniform distribution if no active files match
	if len(pers) == 0 {
		uniform := 1.0 / float64(numNodes)
		for _, n := range nodesMap {
			pers[n.ID()] = uniform
		}
	}
	
	// 2. Pre-calculate out-weight sum for each node to speed up O(1) lookups
	outWeightSum := make(map[int64]float64)
	nodes := g.Graph.Nodes()
	for nodes.Next() {
		u := nodes.Node().ID()
		sum := 0.0
		toNodes := g.Graph.From(u)
		for toNodes.Next() {
			v := toNodes.Node().ID()
			weight, _ := g.Graph.Weight(u, v)
			sum += weight
		}
		outWeightSum[u] = sum
	}

	// 3. Initialize PageRank
	pr := make(map[int64]float64)
	for id, val := range pers {
		pr[id] = val
	}
	
	// 4. Power Iteration loop
	for i := 0; i < iterations; i++ {
		nextPr := make(map[int64]float64)
		
		// Sum rank of dangling nodes (nodes with outWeightSum == 0)
		danglingSum := 0.0
		for _, n := range nodesMap {
			v := n.ID()
			if outWeightSum[v] == 0 {
				danglingSum += pr[v]
			}
		}
		
		nodes := g.Graph.Nodes()
		for nodes.Next() {
			u := nodes.Node().ID()
			
			// Base jump probability + distributed dangling rank
			rank := ((1.0 - d) + d*danglingSum) * pers[u]
			
			// Inbound flow from dependent nodes
			fromNodes := g.Graph.To(u)
			for fromNodes.Next() {
				v := fromNodes.Node().ID()
				weight, _ := g.Graph.Weight(v, u)
				outSum := outWeightSum[v]
				
				if outSum > 0 {
					rank += d * pr[v] * (weight / outSum)
				}
			}
			
			nextPr[u] = rank
		}
		pr = nextPr
	}
	
	// 5. Project back to filepaths
	result := make(map[string]float64)
	for id, rank := range pr {
		filepath := g.idToFn[id]
		result[filepath] = rank
	}
	
	// 6. Apply massive multiplier (50x) for active files to prioritize them
	for _, f := range activeFiles {
		if _, exists := result[f]; exists {
			result[f] *= 50.0
		}
	}
	
	return result
}
