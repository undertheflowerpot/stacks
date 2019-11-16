package reconciler

// nolint: golint
import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/docker/stacks/pkg/fakes"
	"github.com/docker/stacks/pkg/interfaces"
)

// TestBackendClient permits different tests to specify the BackendClient implementation
var TestBackendClient interfaces.BackendClient

// InitAlgSecret shared IT/UNIT test
var InitAlgSecret = Describe("Initial Algorithm Support - Fake Secrets", func() {
	// testing two secrets one with a stacks label and
	// one without skipping zero-th element
	resources := fakes.GenerateSecretFixtures(3, "", "InitialSecret")
	rr1 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[1].Spec.Annotations.Name,
		},
		Config: &resources[1].Spec,
	}
	rr2 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[2].Spec.Annotations.Name,
		},
		Config: &resources[2].Spec,
	}

	SimpleAlgorithmPluginCRDTest(rr1, rr2, interfaces.ReconcileSecret)
})

// InitAlgConfig shared IT/UNIT test
var InitAlgConfig = Describe("Initial Algorithm Support - Fake Configs", func() {
	// testing two configs one with a stacks label and
	// one without skipping zero-th element
	resources := fakes.GenerateConfigFixtures(3, "", "InitialConfig")
	rr1 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[1].Spec.Annotations.Name,
		},
		Config: &resources[1].Spec,
	}
	rr2 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[2].Spec.Annotations.Name,
		},
		Config: &resources[2].Spec,
	}

	SimpleAlgorithmPluginCRDTest(rr1, rr2, interfaces.ReconcileConfig)
})

// InitAlgNetwork shared IT/UNIT test
var InitAlgNetwork = Describe("Initial Algorithm Support - Fake Networks", func() {

	// testing two networks one with a stacks label and
	// one without skipping zero-th element
	resources := fakes.GenerateNetworkFixtures(3, "", "InitialNetwork")
	rr1 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[1].Name,
		},
		Config: &resources[1],
	}
	rr2 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[2].Name,
		},
		Config: &resources[2],
	}

	SimpleAlgorithmPluginCRDTest(rr1, rr2, interfaces.ReconcileNetwork)
})

// InitAlgService shared IT/UNIT test
var InitAlgService = Describe("Initial Algorithm Support - Fake Services", func() {

	// testing two services one with a stacks label and
	// one without skipping zero-th element
	resources := fakes.GenerateServiceFixtures(3, "", "InitialService")
	rr1 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[1].Spec.Annotations.Name,
		},
		Config: &resources[1].Spec,
	}
	rr2 := interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: resources[2].Spec.Annotations.Name,
		},
		Config: &resources[2].Spec,
	}

	SimpleAlgorithmPluginCRDTest(rr1, rr2, interfaces.ReconcileService)
})

// SimpleAlgorithmPluginCRDTest generalized tests all plugins for create, delete and get
func SimpleAlgorithmPluginCRDTest(rr1, rr2 interfaces.ReconcileResource, kind interfaces.ReconcileKind) bool {
	return Describe("Initial Algorithm Support", func() {
		var (
			plugin                AlgorithmPlugin
			pluginWithoutID       AlgorithmPlugin
			inputs                AlgorithmPluginInputs
			err                   error
			initializationSupport InitializationSupport
		)
		BeforeEach(func() {
			switch kind {

			case interfaces.ReconcileService:
				initializationSupport = NewInitializationSupportService(TestBackendClient)
				break
			case interfaces.ReconcileSecret:
				initializationSupport = NewInitializationSupportSecret(TestBackendClient)
				break
			case interfaces.ReconcileNetwork:
				initializationSupport = NewInitializationSupportNetwork(TestBackendClient)
				break
			case interfaces.ReconcileConfig:
				initializationSupport = NewInitializationSupportConfig(TestBackendClient)
				break
			}
			snapshot := interfaces.SnapshotStack{
				SnapshotResource: interfaces.SnapshotResource{
					ID: "foobar",
				},
			}
			snapshotWithoutID := interfaces.SnapshotStack{
				SnapshotResource: interfaces.SnapshotResource{
					ID: "",
				},
			}
			plugin = initializationSupport.CreatePlugin(snapshot, nil)
			pluginWithoutID = initializationSupport.CreatePlugin(snapshotWithoutID, nil)

			inputs.err1 = pluginWithoutID.CreateResource(&rr1)
			inputs.err2 = plugin.CreateResource(&rr2)

			inputs.algorithmInit = initializationSupport
			inputs.search1.SnapshotResource = interfaces.SnapshotResource{
				Name: rr1.Name,
				ID:   rr1.ID,
			}
			inputs.search2.SnapshotResource = interfaces.SnapshotResource{
				Name: rr2.Name,
				ID:   rr2.ID,
			}
			inputs.stackID = snapshot.ID
		})
		AfterEach(func() {
			err1 := plugin.DeleteResource(&rr1)
			err2 := plugin.DeleteResource(&rr2)
			Expect(err1).ToNot(HaveOccurred())
			Expect(err2).ToNot(HaveOccurred())
		})
		When("Lookup non-existent resources", func() {
			BeforeEach(func() {
				badsearch := interfaces.ReconcileResource{}
				inputs.activeResource1, err = inputs.algorithmInit.GetActiveResource(badsearch)
			})
			It("ActiveResource does not exist", func() {
				Expect(err).To(HaveOccurred())
			})
		})
		When("Test resources are created", func() {
			It("The creations do not fail", func() {
				Expect(inputs.err1).ToNot(HaveOccurred())
				Expect(inputs.err2).ToNot(HaveOccurred())
			})
		})
		When("an unlabled resource is created", func() {
			SimplePluginLookupTests(&inputs)
		})
	})
}
