package dispatcher_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/docker/stacks/pkg/fakes"
	"github.com/docker/stacks/pkg/reconciler/reconciler"
)

var _ = BeforeSuite(func() {
	reconciler.TestBackendClient = fakes.NewFakeBackendAPIClientShim()
})

func TestDispatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dispatcher Suite")
}
