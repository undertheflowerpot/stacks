package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/docker/stacks/pkg/controller/standalone"
	"github.com/docker/stacks/pkg/reconciler/reconciler"
)

var Server *standalone.ServerControl

var _ = BeforeSuite(func() {
	var err error
	Server, err = standalone.CreateServer(standalone.ServerOptions{
		Debug:            false,
		DockerSocketPath: socketFlag.Value,
		ServerPort:       portFlag.Value,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		_ = Server.RunServer()
	}()
	reconciler.TestBackendClient = Server.BackendClient
})

var _ = AfterSuite(func() {
	Server.StopServer()
})

func TestReconciler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Suite")
}
