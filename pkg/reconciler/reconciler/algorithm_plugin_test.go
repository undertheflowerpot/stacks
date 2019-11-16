package reconciler

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/docker/stacks/pkg/fakes"
	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

var _ = Describe("Algorithm Plugin for Service - Stack Request", func() {
	var (
		serviceInit    InitializationService
		stack          types.Stack
		input          AlgorithmPluginInputs
		err1, err2     error
		snapshot       interfaces.SnapshotStack
		serviceSupport algorithmService
		stackCreate    types.StackCreateResponse
	)
	BeforeEach(func() {
		input.cli = fakes.NewFakeReconcilerClient()
		serviceInit = NewInitializationSupportService(input.cli)
		stack = fakes.GetTestStack("stack12")
		input.stack = &stack
		input.algorithmInit = serviceInit
		stackCreate, err1 = input.cli.CreateStack(input.stack.Spec)
		input.stackID = stackCreate.ID
		snapshot, err2 = input.cli.GetSnapshotStack(input.stackID)
		input.specName = input.stack.Spec.Services[0].Annotations.Name
	})
	It("Initializations Succeed", func() {
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
	})
	BeforeEach(func() {
		input.request = &interfaces.ReconcileResource{
			SnapshotResource: interfaces.SnapshotResource{
				ID: input.stackID,
			},
			Kind: interfaces.ReconcileStack,
		}

		serviceSupport := newAlgorithmPluginService(serviceInit, snapshot, input.request)
		input.resourcePlugin = serviceSupport
	})
	Context("With a fresh plugin", func() {
		It("Coverage missing lookup", func() {
			noSpecName := serviceSupport.lookupServiceSpec("missing")
			Expect(noSpecName).To(BeNil())
		})
		FreshPluginAssertions(&input)
	})
	Context("Add a creation goal", func() {
		CreatedGoalAssertions(&input)

		Context("Manually reconcile goal", func() {
			ManuallyReconcileGoal(&input)

			Context("Manually store goal", func() {
				ManuallyStoreGoal(&input)
			})
			Context("Manually update goal", func() {
				ManuallyReconcileUpdateGoal(&input)
			})
			Context("Manually remove goal", func() {
				ManuallyRemoveGoal(&input)
				Context("Manually remove goal", func() {
					ManuallyReconcileRemoveGoal(&input)
				})
			})
		})
	})
	Context("Provide an update scenario", func() {
		BeforeEach(func() {
			CreatedGoalAssertions(&input)
			ManuallyReconcileGoal(&input)
			ManuallyStoreGoal(&input)
			/* for another tests
			// Manual alteration of underlying service
			input.stack.Spec.Services[0].UpdateConfig =
				&swarm.UpdateConfig{}
			service, _ := input.cli.GetService(input.activeResource.GetSnapshot().ID,
						interfaces.DefaultGetServiceArg2)
			input.cli.UpdateService(service.ID,
						service.Meta.Version.Index,
						input.stack.Spec.Services[0],
						interfaces.DefaultUpdateServiceArg4,
						interfaces.DefaultUpdateServiceArg5)
			snapshot, err2 = input.cli.GetSnapshotStack(input.stackID)
			serviceSupport := newAlgorithmPluginService(serviceInit, snapshot, input.request)
			input.resourcePlugin = serviceSupport
			*/

		})

	})
})

var _ = Describe("Algorithm Plugin for Config - Stack Request", func() {
	var (
		configInit    InitializationConfig
		stack         types.Stack
		input         AlgorithmPluginInputs
		err1, err2    error
		snapshot      interfaces.SnapshotStack
		configSupport algorithmConfig
		stackCreate   types.StackCreateResponse
	)
	BeforeEach(func() {
		input.cli = fakes.NewFakeReconcilerClient()
		configInit = NewInitializationSupportConfig(input.cli)
		stack = fakes.GetTestStack("stack12")
		input.stack = &stack
		input.algorithmInit = configInit
		stackCreate, err1 = input.cli.CreateStack(input.stack.Spec)
		input.stackID = stackCreate.ID
		snapshot, err2 = input.cli.GetSnapshotStack(input.stackID)
		input.specName = input.stack.Spec.Configs[0].Annotations.Name
	})
	It("Initializations Succeed", func() {
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
	})
	BeforeEach(func() {
		input.request = &interfaces.ReconcileResource{
			SnapshotResource: interfaces.SnapshotResource{
				ID: input.stackID,
			},
			Kind: interfaces.ReconcileStack,
		}

		configSupport := newAlgorithmPluginConfig(configInit, snapshot, input.request)
		input.resourcePlugin = configSupport
	})
	Context("With a fresh plugin", func() {
		It("Coverage missing lookup", func() {
			noSpecName := configSupport.lookupConfigSpec("missing")
			Expect(noSpecName).To(BeNil())
		})
		FreshPluginAssertions(&input)

		Context("Add a creation goal", func() {
			CreatedGoalAssertions(&input)

			Context("Manually reconcile goal", func() {
				ManuallyReconcileGoal(&input)

				Context("Manually store goal", func() {
					ManuallyStoreGoal(&input)
				})
				Context("Manually update goal", func() {
					ManuallyReconcileUpdateGoal(&input)
				})
				Context("Manually remove goal", func() {
					ManuallyRemoveGoal(&input)
					Context("Manually remove goal", func() {
						ManuallyReconcileRemoveGoal(&input)
					})
				})
			})
		})
	})
})

var _ = Describe("Algorithm Plugin for Secret - Stack Request", func() {
	var (
		secretInit    InitializationSecret
		stack         types.Stack
		input         AlgorithmPluginInputs
		err1, err2    error
		snapshot      interfaces.SnapshotStack
		secretSupport algorithmSecret
		stackCreate   types.StackCreateResponse
	)
	BeforeEach(func() {
		input.cli = fakes.NewFakeReconcilerClient()
		secretInit = NewInitializationSupportSecret(input.cli)
		stack = fakes.GetTestStack("stack12")
		input.stack = &stack
		input.algorithmInit = secretInit
		stackCreate, err1 = input.cli.CreateStack(input.stack.Spec)
		input.stackID = stackCreate.ID
		snapshot, err2 = input.cli.GetSnapshotStack(input.stackID)
		input.specName = input.stack.Spec.Secrets[0].Annotations.Name
	})
	It("Initializations Succeed", func() {
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
	})
	BeforeEach(func() {
		input.request = &interfaces.ReconcileResource{
			SnapshotResource: interfaces.SnapshotResource{
				ID: input.stackID,
			},
			Kind: interfaces.ReconcileStack,
		}

		secretSupport := newAlgorithmPluginSecret(secretInit, snapshot, input.request)
		input.resourcePlugin = secretSupport
	})
	Context("With a fresh plugin", func() {
		It("Coverage missing lookup", func() {
			noSpecName := secretSupport.lookupSecretSpec("missing")
			Expect(noSpecName).To(BeNil())
		})
		FreshPluginAssertions(&input)

		Context("Add a creation goal", func() {
			CreatedGoalAssertions(&input)

			Context("Manually reconcile goal", func() {
				ManuallyReconcileGoal(&input)

				Context("Manually store goal", func() {
					ManuallyStoreGoal(&input)
				})
				Context("Manually update goal", func() {
					ManuallyReconcileUpdateGoal(&input)
				})
				Context("Manually remove goal", func() {
					ManuallyRemoveGoal(&input)
					Context("Manually remove goal", func() {
						ManuallyReconcileRemoveGoal(&input)
					})
				})
			})
		})
	})
})

var _ = Describe("Algorithm Plugin for Network - Stack Request", func() {
	var (
		networkInit    InitializationNetwork
		stack          types.Stack
		input          AlgorithmPluginInputs
		err1, err2     error
		snapshot       interfaces.SnapshotStack
		networkSupport algorithmNetwork
		stackCreate    types.StackCreateResponse
	)
	BeforeEach(func() {
		input.cli = fakes.NewFakeReconcilerClient()
		networkInit = NewInitializationSupportNetwork(input.cli)
		stack = fakes.GetTestStack("stack12")
		input.stack = &stack
		input.algorithmInit = networkInit
		stackCreate, err1 = input.cli.CreateStack(input.stack.Spec)
		input.stackID = stackCreate.ID
		snapshot, err2 = input.cli.GetSnapshotStack(input.stackID)
		for networkName := range input.stack.Spec.Networks {
			input.specName = networkName
			break
		}
	})
	It("Initializations Succeed", func() {
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
	})
	BeforeEach(func() {
		input.request = &interfaces.ReconcileResource{
			SnapshotResource: interfaces.SnapshotResource{
				ID: input.stackID,
			},
			Kind: interfaces.ReconcileStack,
		}

		networkSupport := newAlgorithmPluginNetwork(networkInit, snapshot, input.request)
		input.resourcePlugin = networkSupport
	})
	Context("With a fresh plugin", func() {
		It("Coverage missing lookup", func() {
			noSpecName := networkSupport.lookupNetworkSpec("missing")
			Expect(noSpecName).To(BeNil())
		})
		FreshPluginAssertions(&input)

		Context("Add a creation goal", func() {
			CreatedGoalAssertions(&input)

			Context("Manually reconcile goal", func() {
				ManuallyReconcileGoal(&input)

				Context("Manually store goal", func() {
					ManuallyStoreGoal(&input)
				})
				Context("Manually update goal", func() {
					ManuallyReconcileUpdateGoal(&input)
				})
				Context("Manually remove goal", func() {
					ManuallyRemoveGoal(&input)
					Context("Manually remove goal", func() {
						ManuallyReconcileRemoveGoal(&input)
					})
				})
			})
		})
	})
})
