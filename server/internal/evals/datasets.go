package evals

import (
	"context"
	"fmt"
	"sync"
)

type GoldenCase struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Input            string            `json:"input"`
	ExpectedBehavior string            `json:"expected_behavior"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type Dataset struct {
	Name  string       `json:"name"`
	Cases []GoldenCase `json:"cases"`
}

type DatasetStore interface {
	Get(ctx context.Context, name string) (Dataset, error)
	Save(ctx context.Context, dataset Dataset) error
}

type MemoryDatasetStore struct {
	mu       sync.RWMutex
	datasets map[string]Dataset
}

func NewMemoryDatasetStore(datasets ...Dataset) *MemoryDatasetStore {
	store := &MemoryDatasetStore{datasets: map[string]Dataset{}}
	for _, dataset := range datasets {
		store.datasets[dataset.Name] = dataset
	}
	return store
}

func (s *MemoryDatasetStore) Get(_ context.Context, name string) (Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dataset, ok := s.datasets[name]
	if !ok {
		return Dataset{}, fmt.Errorf("dataset %q not found", name)
	}
	return dataset, nil
}

func (s *MemoryDatasetStore) Save(_ context.Context, dataset Dataset) error {
	if dataset.Name == "" {
		return fmt.Errorf("dataset name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.datasets[dataset.Name] = dataset
	return nil
}
