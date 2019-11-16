package standalone

import (
	"context"
	"fmt"
	"net/http"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/server/router"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/docker/stacks/pkg/controller/backend"
	stacksRouter "github.com/docker/stacks/pkg/controller/router"
	"github.com/docker/stacks/pkg/fakes"
	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/reconciler"
)

// ServerOptions is the set of options required for the creation of a
// standalone.Server instance.
type ServerOptions struct {
	Debug            bool
	DockerSocketPath string
	ServerPort       int
}

// ServerControl makes it possible to pass control of the standalone
// httpserver to Ginko tests.
type ServerControl struct {
	StackStore           *fakes.FakeStackStore
	SwarmResourceBackend interfaces.SwarmResourceBackend
	StacksBackend        *backend.DefaultStacksBackend
	BackendClient        interfaces.BackendClient
	ReconcilerManager    *reconciler.Manager
	Server               *http.Server
}

// CreateServer initializes a standalone http Server that serves the Stacks
// API, and sets up the Stacks reconciler. A docker API client, accessible as a
// unix socket via the DockerSocketPath option, provides access to the
// downstream set of Swarmkit required for the reconciler.
func CreateServer(opts ServerOptions) (*ServerControl, error) {
	if opts.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Create an unauthenticated docker client
	dclient, err := client.NewClient(fmt.Sprintf("unix://%s", opts.DockerSocketPath), "", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create docker client for unix socket at %s: %s", opts.DockerSocketPath, err)
	}

	s := ServerControl{}

	// Create a shim for the SwarmResourceBackend interface using the docker client.
	// This shim is used to access swarm resources by the Stacks API handlers
	// for validation and conversion purposes.
	s.SwarmResourceBackend = interfaces.NewSwarmAPIClientShim(dclient)

	// Create the underlying storage for stacks and swarmstacks as an
	// in-memory store.
	s.StackStore = fakes.NewFakeStackStore()

	// Create a Stacks API Backend, which includes the API handling logic.
	s.StacksBackend = backend.NewDefaultStacksBackend(s.StackStore, s.SwarmResourceBackend)

	// Create a BackendClient shim for the reconciler
	s.BackendClient = interfaces.NewBackendAPIClientShim(dclient, s.StacksBackend)

	// Create the reconciler
	s.ReconcilerManager = reconciler.New(s.BackendClient)

	// Create a Stacks API Router, which includes basic HTTP handlers
	// for the Stacks APIs. This is wired up against the backendClient
	// so that the API can trigger stack events.
	r := stacksRouter.NewRouter(s.BackendClient)

	s.Server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", opts.ServerPort),
		Handler: registerRoutes(r),
	}
	return &s, nil
}

// RunServer takes the Server created from CreateServer and starts the
// goroutines to run the httpserver
func (s *ServerControl) RunServer() error {
	errChan := make(chan error)

	// Launch the reconciler in a goroutine
	go func() {
		logrus.Infof("Starting Swarm Stacks reconciler")
		errChan <- s.ReconcilerManager.Run()
	}()

	// Launch the HTTP server in a goroutine
	go func() {
		logrus.Infof("Running standalone Stacks API server")
		errChan <- s.Server.ListenAndServe()
	}()

	return <-errChan
}

// StopServer shutsdown the httpserver
func (s *ServerControl) StopServer() error {
	return s.Server.Shutdown(context.Background())
}

// versionMatcher defines a variable matcher to be parsed by the router
// when a request is about to be served.
const versionMatcher = "/v{version:[0-9.]+}"

// Implementation loosely based on
// https://github.com/moby/moby/blob/master/api/server/server.go#L171-L198
func registerRoutes(r router.Router) http.Handler {
	m := mux.NewRouter()
	for _, r := range r.Routes() {
		f := makeHTTPHandler(r.Handler())
		m.Path(versionMatcher + r.Path()).Methods(r.Method()).Handler(f)
		m.Path(r.Path()).Methods(r.Method()).Handler(f)
	}

	return m
}

func makeHTTPHandler(handler httputils.APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		if vars == nil {
			vars = make(map[string]string)
		}

		if err := handler(r.Context(), w, r, vars); err != nil {
			statusCode := httputils.GetHTTPErrorStatusCode(err)
			if statusCode >= 500 {
				logrus.Errorf("Handler for %s %s returned error: %v", r.Method, r.URL.Path, err)
			}
			httputils.MakeErrorHandler(err)(w, r)
		}
	}
}
