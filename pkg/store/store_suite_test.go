package store_test

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

func TestStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Store Suite")
}
