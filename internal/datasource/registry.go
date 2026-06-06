package datasource

import (
	"fmt"
	"slices"
	"sync"
)

// Registry manages available datasource drivers.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// NewRegistry creates a new driver registry.
func NewRegistry() *Registry {
	return &Registry{
		drivers: make(map[string]Driver),
	}
}

// Register adds a driver to the registry.
func (r *Registry) Register(driver Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[driver.Type()] = driver
}

// Get returns a driver by type name.
func (r *Registry) Get(typeName string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	driver, ok := r.drivers[typeName]
	if !ok {
		return nil, fmt.Errorf("unsupported datasource type: %s", typeName)
	}
	return driver, nil
}

// List returns all registered driver type names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.drivers))
	for t := range r.drivers {
		types = append(types, t)
	}
	slices.Sort(types)
	return types
}
