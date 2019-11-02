package interfaces

import (
	"context"
	"fmt"
	"sync"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"

	"github.com/docker/stacks/pkg/types"
)

// BackendAPIClientShim is an implementation of BackendClient that utilizes an
// in-memory FakeStackStore for Stacks CRUD, and an underlying Docker API
// Client for swarm operations. It is intended for use only as part of the
// standalone runtime of the stacks controller. Only one event subscriber is
// expected at any time.
type BackendAPIClientShim struct {
	dclient client.CommonAPIClient
	StacksBackend

	SwarmResourceBackend

	// The following constructs are used to generate events for stack
	// operations locally, and multiplex them into the daemon's event stream.
	stackEvents   chan events.Message
	subscribersMu sync.Mutex
	subscribers   map[chan interface{}]context.CancelFunc
}

// NewBackendAPIClientShim creates a new BackendAPIClientShim.
func NewBackendAPIClientShim(dclient client.CommonAPIClient, backend StacksBackend) BackendClient {
	return &BackendAPIClientShim{
		dclient:              dclient,
		StacksBackend:        backend,
		SwarmResourceBackend: NewSwarmAPIClientShim(dclient),
		stackEvents:          make(chan events.Message),
		subscribers:          make(map[chan interface{}]context.CancelFunc),
	}
}

// SubscribeToEvents subscribes to the system event stream. The API Client's
// Events API has no way to distinguish between buffered and streamed events,
// thus even past are provided through the returned channel.
func (c *BackendAPIClientShim) SubscribeToEvents(since, until time.Time, ef filters.Args) ([]events.Message, chan interface{}) {
	ctx, cancel := context.WithCancel(context.Background())
	logrus.Infof("SHIM:SubscribeToEvents since=%v until=%v", since, until)

	resChan := make(chan interface{})
	eventsChan, errChan := c.dclient.Events(context.Background(), dockerTypes.EventsOptions{
		Filters: ef,
		//		Since:   fmt.Sprintf("%d", since.Unix()),
		//		Until:   fmt.Sprintf("%d", until.Unix()),
	})

	go func() {
		for {
			select {
			case err := <-errChan:
				logrus.Errorf("SHIM:SubscribeToEvents daemon error: %v", err)
			case event := <-c.stackEvents:
				resChan <- event
			case event := <-eventsChan:
				resChan <- event
			case <-ctx.Done():
				return
			}
		}
	}()

	c.subscribersMu.Lock()
	c.subscribers[resChan] = cancel
	c.subscribersMu.Unlock()

	return []events.Message{}, resChan
}

// UnsubscribeFromEvents unsubscribes from the event stream.
func (c *BackendAPIClientShim) UnsubscribeFromEvents(eventChan chan interface{}) {
	c.subscribersMu.Lock()
	defer c.subscribersMu.Unlock()

	if cancelFunc, ok := c.subscribers[eventChan]; ok {
		cancelFunc()
		delete(c.subscribers, eventChan)
	}
}

// CreateStack creates a stack
func (c *BackendAPIClientShim) CreateStack(create types.StackSpec) (types.StackCreateResponse, error) {
	response, err := c.StacksBackend.CreateStack(create)
	if err != nil {
		return response, fmt.Errorf("unable to create stack: %s", err)
	}

	go func() {
		ev := events.Message{
			Type:   "stack",
			Action: "create",
			Actor: events.Actor{
				ID: response.ID,
			},
		}
		logrus.Debugf("SHIM:CreateStack writing event: %v", ev)
		c.stackEvents <- ev
		logrus.Debugf("SHIM:CreateStack wrote event")
	}()

	return response, err
}

// UpdateStack updates a stack.
func (c *BackendAPIClientShim) UpdateStack(id string, spec types.StackSpec, version uint64) error {
	err := c.StacksBackend.UpdateStack(id, spec, version)
	go func() {
		ev := events.Message{
			Type:   "stack",
			Action: "update",
			Actor: events.Actor{
				ID: id,
			},
		}
		logrus.Debugf("SHIM:UpdateStack writing event: %v", ev)
		c.stackEvents <- ev
		logrus.Debugf("SHIM:UpdateStack wrote event")
	}()

	return err
}

// DeleteStack deletes a stack.
func (c *BackendAPIClientShim) DeleteStack(id string) error {
	err := c.StacksBackend.DeleteStack(id)
	go func() {
		ev := events.Message{
			Type:   "stack",
			Action: "delete",
			Actor: events.Actor{
				ID: id,
			},
		}
		logrus.Debugf("SHIM:DeleteStack writing event: %v", ev)
		c.stackEvents <- ev
		logrus.Debugf("SHIM:DeleteStack wrote event")
	}()
	return err
}
