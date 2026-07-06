package repomap

import (
	"testing"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

func TestGraphAndPageRank(t *testing.T) {
	tags := []source.Tag{
		{Name: "User", Kind: "def", Filepath: "models.go"},
		{Name: "Login", Kind: "def", Filepath: "auth.go"},
		{Name: "User", Kind: "ref", Filepath: "auth.go"},  // auth.go depends on models.go
		{Name: "Login", Kind: "ref", Filepath: "main.go"}, // main.go depends on auth.go
	}
	
	g := NewDependencyGraph()
	g.BuildGraph(tags)
	
	// Test edges
	if g.Graph.Nodes().Len() != 3 {
		t.Fatalf("Expected 3 nodes in graph, got %d", g.Graph.Nodes().Len())
	}
	
	// Test Personalized PageRank logic
	active := []string{"main.go"}
	pr := g.CalculatePageRank(active)
	
	if len(pr) != 3 {
		t.Fatal("Expected ranks for exactly 3 files")
	}
	
	// A random surfer starting heavily at main.go will visit auth.go, then models.go.
	// Therefore, models.go must receive a non-zero rank.
	if pr["models.go"] == 0 {
		t.Error("PageRank failed to flow to dependencies: models.go has 0 score")
	}
	if pr["auth.go"] == 0 {
		t.Error("PageRank failed to flow to dependencies: auth.go has 0 score")
	}
}

func TestPageRankDanglingNode(t *testing.T) {
	// A -> B. B is dangling (has no outbound edges).
	tags := []source.Tag{
		{Name: "BFunc", Kind: "def", Filepath: "B.go"},
		{Name: "BFunc", Kind: "ref", Filepath: "A.go"},
	}

	g := NewDependencyGraph()
	g.BuildGraph(tags)

	pr := g.CalculatePageRank([]string{"A.go"})
	
	// Sum of all ranks must equal 1.0 (rank conservation)
	sum := 0.0
	for _, val := range pr {
		sum += val
	}

	if sum < 0.999 || sum > 1.001 {
		t.Errorf("PageRank sum of dangling nodes leaked. Expected ~1.0, got %f", sum)
	}
}

func TestDuplicateSymbolRouting(t *testing.T) {
	tags := []source.Tag{
		// Target 1: closer to app/main.go
		{Name: "app/pkg1/pkg1.go: Init", Kind: "def", Filepath: "app/pkg1/pkg1.go"},
		// Target 2: farther from app/main.go
		{Name: "other/pkg2/pkg2.go: Init", Kind: "def", Filepath: "other/pkg2/pkg2.go"},
		// Reference in app/main.go
		{Name: "Init", Kind: "ref", Filepath: "app/main.go"},
	}

	g := NewDependencyGraph()
	g.BuildGraph(tags)

	// Check that edge A -> B is created between app/main.go and app/pkg1/pkg1.go
	srcNode := g.nodes["app/main.go"]
	targetNodeClose := g.nodes["app/pkg1/pkg1.go"]
	targetNodeFar := g.nodes["other/pkg2/pkg2.go"]

	if srcNode == nil || targetNodeClose == nil || targetNodeFar == nil {
		t.Fatal("Nodes were not added correctly to the graph")
	}

	if !g.Graph.HasEdgeFromTo(srcNode.ID(), targetNodeClose.ID()) {
		t.Error("Expected edge from app/main.go to closest target app/pkg1/pkg1.go")
	}

	if g.Graph.HasEdgeFromTo(srcNode.ID(), targetNodeFar.ID()) {
		t.Error("Did not expect edge from app/main.go to farther target other/pkg2/pkg2.go")
	}
}
