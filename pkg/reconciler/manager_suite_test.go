package reconciler

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

func TestManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manager Suite")
}
