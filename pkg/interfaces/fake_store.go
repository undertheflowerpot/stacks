package interfaces

import (
	"errors"
	"fmt"
	"sync"

	"github.com/docker/stacks/pkg/types"
)

// stackPair is a pair of a stack and a swarmStack
type stackPair struct {
	types.Stack
	SwarmStack
}

// FakeStackStore stores stacks
type FakeStackStore struct {
	stacks map[string]stackPair
	sync.RWMutex
	curID int
}

// NewFakeStackStore creates a new StackStore
func NewFakeStackStore() StackStore {
	return &FakeStackStore{
		stacks: make(map[string]stackPair),
		// Don't start from ID 0, to catch any uninitialized types.
		curID: 1,
	}
}

var errNotFound = errors.New("stack not found")

// IsErrNotFound return true if the error is a not-found error.
func IsErrNotFound(err error) bool {
	return err == errNotFound
}

// AddStack adds a stack to the store.
func (s *FakeStackStore) AddStack(stack types.Stack, swarmStack SwarmStack) (string, error) {
	s.Lock()
	defer s.Unlock()

	stack.ID = fmt.Sprintf("%d", s.curID)
	swarmStack.ID = stack.ID
	s.stacks[stack.ID] = stackPair{
		Stack:      stack,
		SwarmStack: swarmStack,
	}
	s.curID++
	return stack.ID, nil
}

func (s *FakeStackStore) getStack(id string) (stackPair, error) {
	stack, ok := s.stacks[id]
	if !ok {
		return stackPair{}, errNotFound
	}

	return stack, nil
}

// UpdateStack updates the stack in the store.
func (s *FakeStackStore) UpdateStack(id string, spec types.StackSpec, swarmSpec SwarmStackSpec) error {
	s.Lock()
	defer s.Unlock()

	existingStack, err := s.getStack(id)
	if err != nil {
		return errNotFound
	}

	existingStack.Stack.Spec = spec
	existingStack.SwarmStack.Spec = swarmSpec
	s.stacks[id] = existingStack

	return nil
}

// DeleteStack removes a stack from the store.
func (s *FakeStackStore) DeleteStack(id string) error {
	s.Lock()
	defer s.Unlock()
	delete(s.stacks, id)
	return nil
}

// GetStack retrieves a single stack from the store.
func (s *FakeStackStore) GetStack(name string) (types.Stack, error) {
	s.RLock()
	defer s.RUnlock()
	stackPair, err := s.getStack(name)
	return stackPair.Stack, err
}

// GetSwarmStack retrieves a single swarm stack from the store.
func (s *FakeStackStore) GetSwarmStack(name string) (SwarmStack, error) {
	s.RLock()
	defer s.RUnlock()
	stackPair, err := s.getStack(name)
	return stackPair.SwarmStack, err
}

// ListStacks returns all known stacks from the store.
func (s *FakeStackStore) ListStacks() ([]types.Stack, error) {
	s.RLock()
	defer s.RUnlock()
	stacks := []types.Stack{}
	for _, stack := range s.stacks {
		stacks = append(stacks, stack.Stack)
	}
	return stacks, nil
}

// ListSwarmStacks returns all known swarm stacks from the store.
func (s *FakeStackStore) ListSwarmStacks() ([]SwarmStack, error) {
	s.RLock()
	defer s.RUnlock()
	stacks := []SwarmStack{}
	for _, stack := range s.stacks {
		stacks = append(stacks, stack.SwarmStack)
	}
	return stacks, nil
}
