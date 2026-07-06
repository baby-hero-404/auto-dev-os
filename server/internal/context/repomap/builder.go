package repomap

import (
	"math"
	"path/filepath"
	"strings"

	"gonum.org/v1/gonum/graph/simple"
	"github.com/auto-code-os/auto-code-os/server/internal/context/source"
)

// FileNode implements gonum's graph.Node
type FileNode struct {
	id       int64
	Filepath string
}

func (n FileNode) ID() int64 { return n.id }

// DependencyGraph builds and stores the mathematical relationship between files.
type DependencyGraph struct {
	Graph  *simple.WeightedDirectedGraph
	nodes  map[string]*FileNode
	idToFn map[int64]string
	nextID int64
}

func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Graph:  simple.NewWeightedDirectedGraph(0, 0),
		nodes:  make(map[string]*FileNode),
		idToFn: make(map[int64]string),
	}
}

func (g *DependencyGraph) addNodeIfMissing(filepath string) *FileNode {
	if n, exists := g.nodes[filepath]; exists {
		return n
	}
	n := &FileNode{id: g.nextID, Filepath: filepath}
	g.nextID++
	g.nodes[filepath] = n
	g.idToFn[n.id] = filepath
	g.Graph.AddNode(n)
	return n
}

// BuildGraph constructs a directed graph of files based on their def/ref tags.
// Edge A -> B indicates File A calls File B. Weight is sqrt(calls).
func (g *DependencyGraph) BuildGraph(tags []source.Tag) {
	// 1. Map Definitions
	defs := make(map[string][]string)
	for _, t := range tags {
		if t.Kind == "def" {
			parts := strings.SplitN(t.Name, ": ", 2)
			baseName := parts[len(parts)-1]
			defs[baseName] = append(defs[baseName], t.Filepath)
			g.addNodeIfMissing(t.Filepath)
		}
	}

	// 2. Count References (A -> B)
	refCounts := make(map[string]map[string]int)
	for _, t := range tags {
		if t.Kind == "ref" {
			sourceFile := t.Filepath
			g.addNodeIfMissing(sourceFile)

			targetFiles, exists := defs[t.Name]
			if exists {
				bestTarget := ""
				maxCommon := -1
				for _, targetFile := range targetFiles {
					if targetFile == sourceFile {
						continue
					}
					common := commonPrefixLength(sourceFile, targetFile)
					if common > maxCommon {
						maxCommon = common
						bestTarget = targetFile
					}
				}

				if bestTarget != "" {
					if refCounts[sourceFile] == nil {
						refCounts[sourceFile] = make(map[string]int)
					}
					refCounts[sourceFile][bestTarget]++
				}
			}
		}
	}

	// 3. Populate Edges with math.Sqrt(weight)
	for srcFile, targets := range refCounts {
		srcNode := g.nodes[srcFile]
		for targetFile, count := range targets {
			targetNode := g.nodes[targetFile]
			weight := math.Sqrt(float64(count))
			
			edge := g.Graph.WeightedEdge(srcNode.ID(), targetNode.ID())
			if edge == nil {
				g.Graph.SetWeightedEdge(simple.WeightedEdge{
					F: srcNode,
					T: targetNode,
					W: weight,
				})
			} else {
				g.Graph.SetWeightedEdge(simple.WeightedEdge{
					F: srcNode,
					T: targetNode,
					W: edge.Weight() + weight,
				})
			}
		}
	}
}

// commonPrefixLength calculates the number of shared directory segments between two file paths.
func commonPrefixLength(p1, p2 string) int {
	s1 := strings.Split(filepath.Dir(p1), string(filepath.Separator))
	s2 := strings.Split(filepath.Dir(p2), string(filepath.Separator))
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}
	common := 0
	for i := 0; i < minLen; i++ {
		if s1[i] == s2[i] {
			common++
		} else {
			break
		}
	}
	return common
}
