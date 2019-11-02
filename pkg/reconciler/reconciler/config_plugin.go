package reconciler

import (
	"reflect"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

type activeConfig struct {
	interfaces.SnapshotResource
	requestedResource *interfaces.ReconcileResource // nolint: unused
	config            swarm.Config
	stackID           string
}

// InitializationConfig is the InitializationSupport variant for swarm.Config
type InitializationConfig struct {
	cli interfaces.BackendClient
}

type algorithmConfig struct {
	InitializationConfig
	requestedResource *interfaces.ReconcileResource
	stackID           string
	stackSpec         types.StackSpec
	goals             map[string]*interfaces.ReconcileResource
}

func (a activeConfig) GetSnapshot() interfaces.SnapshotResource {
	return a.SnapshotResource
}

func (a activeConfig) GetStackID() string {
	return a.stackID
}

// GetActiveResource returns ActiveResource for swarm.Config in interfaces.ReconcileResource
func (i InitializationConfig) GetActiveResource(resource interfaces.ReconcileResource) (ActiveResource, error) {
	config, err := i.cli.GetConfig(resource.ID)
	if err != nil {
		return activeConfig{}, err
	}
	return i.wrapConfig(config), nil
}

func (i InitializationConfig) getSnapshotResourceNames(snapshot interfaces.SnapshotStack) []string {
	result := make([]string, 0, len(snapshot.Configs))
	for _, snapshotResource := range snapshot.Configs {
		result = append(result, snapshotResource.Name)
	}
	return result
}

func (i InitializationConfig) wrapConfig(config swarm.Config) ActiveResource {
	stackID, ok := config.Spec.Annotations.Labels[types.StackLabel]
	if !ok {
		stackID = ""
	}
	return activeConfig{
		SnapshotResource: interfaces.SnapshotResource{
			ID:   config.ID,
			Meta: config.Meta,
			Name: config.Spec.Name,
		},
		config:  config,
		stackID: stackID,
	}
}

// GetKind returns interfaces.ReconcileConfig
func (i InitializationConfig) GetKind() interfaces.ReconcileKind {
	return interfaces.ReconcileConfig
}

// CreatePlugin creates algorithmConfig
func (i InitializationConfig) CreatePlugin(snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) AlgorithmPlugin {
	return newAlgorithmPluginConfig(i, snapshot, requestedResource)
}

// NewInitializationSupportConfig creates InitializationConfig
func NewInitializationSupportConfig(cli interfaces.BackendClient) InitializationConfig {
	return InitializationConfig{
		cli: cli,
	}
}

func newAlgorithmPluginConfig(initConfig InitializationConfig, snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) *algorithmConfig {
	result := algorithmConfig{
		InitializationConfig: initConfig,
		requestedResource:    requestedResource,
		stackID:              snapshot.ID,
		stackSpec:            snapshot.CurrentSpec,
		goals:                map[string]*interfaces.ReconcileResource{},
	}

	for _, resource := range snapshot.Configs {
		result.goals[resource.Name] = transform(resource, &result)
	}

	return &result
}

func (a *algorithmConfig) lookupSpecifiedResource(name string) interface{} {
	return a.lookupConfigSpec(name)
}

func (a *algorithmConfig) getRequestedResource() *interfaces.ReconcileResource {
	return a.requestedResource
}

func (a *algorithmConfig) reconcile(stack interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	return reconcileResource(stack, a)
}

func (a *algorithmConfig) lookupConfigSpec(name string) *swarm.ConfigSpec {
	for _, configSpec := range a.stackSpec.Configs {
		if name == configSpec.Annotations.Name {
			return &configSpec
		}
	}
	return nil
}

func (a *algorithmConfig) getGoalResources() []*interfaces.ReconcileResource {
	result := make([]*interfaces.ReconcileResource, 0, len(a.goals))
	for _, configResource := range a.goals {
		result = append(result, configResource)
	}
	return result
}

func (a *algorithmConfig) getSpecifiedResourceNames() []string {
	result := make([]string, 0, len(a.stackSpec.Configs))
	for _, configSpec := range a.stackSpec.Configs {
		result = append(result, configSpec.Annotations.Name)
	}
	return result
}

// GetActiveResources returns ActiveResource array for swarm.Configs belonging to the stack
func (a *algorithmConfig) GetActiveResources() ([]ActiveResource, error) {
	configs, err := a.cli.GetConfigs(dockerTypes.ConfigListOptions{
		Filters: stackLabelFilter(a.stackID),
	})
	if err != nil {
		return []ActiveResource{}, err
	}
	result := make([]ActiveResource, 0, len(configs))
	for _, config := range configs {
		result = append(result, a.wrapConfig(config))
	}
	return result, nil
}

func (a *algorithmConfig) getGoalResource(name string) *interfaces.ReconcileResource {
	for _, configResource := range a.goals {
		if name == configResource.Name {
			return configResource
		}
	}
	return nil
}

func (a *algorithmConfig) addCreateResourceGoal(specName string) *interfaces.ReconcileResource {
	// returning nil configSpec will generate a panic but that is a bug in the
	// calling code
	configSpec := a.lookupConfigSpec(specName)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: configSpec.Annotations.Name,
		},
		Config: configSpec,
		Kind:   a.GetKind(),
	}
	a.goals[specName] = resource
	return resource
}

func (a *algorithmConfig) addRemoveResourceGoal(activeResource ActiveResource) *interfaces.ReconcileResource {
	activeConfig := activeResource.(activeConfig)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: activeConfig.SnapshotResource,
		Kind:             a.GetKind(),
		Config:           activeConfig.config.Spec,
	}
	a.goals[activeConfig.Name] = resource
	return resource
}

func (a *algorithmConfig) storeGoals(previous interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	goalConfigs := []interfaces.SnapshotResource{}
	for _, resource := range a.goals {
		if resource.Mark == interfaces.ReconcileDelete {
			continue
		}
		goalConfigs = append(goalConfigs, resource.SnapshotResource)
	}

	// Simple copy + override
	updated := previous
	updated.Configs = goalConfigs

	current, err := a.cli.UpdateSnapshotStack(a.stackID,
		updated,
		updated.Meta.Version.Index)
	if err != nil {
		return previous, err
	}

	return current, nil
}

func (a *algorithmConfig) hasSameConfiguration(resource interfaces.ReconcileResource, actual ActiveResource) bool {
	one := resource.Config.(*swarm.ConfigSpec)
	two := actual.(activeConfig).config.Spec
	return one.Annotations.Name == two.Annotations.Name &&
		compareMapsIgnoreStackLabel(one.Annotations.Labels, two.Annotations.Labels) &&
		reflect.DeepEqual(one.Data, two.Data) &&
		reflect.DeepEqual(one.Templating, two.Templating)
}

func (a *algorithmConfig) CreateResource(resource *interfaces.ReconcileResource) error {
	configSpec := resource.Config.(*swarm.ConfigSpec)
	if configSpec.Annotations.Labels == nil {
		configSpec.Annotations.Labels = map[string]string{}
	}
	configSpec.Annotations.Labels[types.StackLabel] = a.stackID
	id, err := a.cli.CreateConfig(*configSpec)
	if err != nil {
		return err
	}
	resource.ID = id
	return nil
}

func (a *algorithmConfig) DeleteResource(resource *interfaces.ReconcileResource) error {
	err := a.cli.RemoveConfig(resource.ID)
	// Ignore not found error
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	resource.ID = ""
	return nil
}

func (a *algorithmConfig) UpdateResource(resource interfaces.ReconcileResource) error {
	// the response from UpdateConfig is irrelevant
	err := a.cli.UpdateConfig(
		resource.ID,
		resource.Meta.Version.Index,
		*resource.Config.(*swarm.ConfigSpec))
	if err != nil {
		return err
	}
	return nil
}
