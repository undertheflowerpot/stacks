package reconciler

import (
	"reflect"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

type activeService struct {
	interfaces.SnapshotResource
	service swarm.Service
	stackID string
}

// InitializationService is the InitializationSupport variant for swarm.Service
type InitializationService struct {
	cli interfaces.BackendClient
}

type algorithmService struct {
	InitializationService
	requestedResource *interfaces.ReconcileResource
	stackID           string
	stackSpec         types.StackSpec
	goals             map[string]*interfaces.ReconcileResource
}

func (a activeService) GetSnapshot() interfaces.SnapshotResource {
	return a.SnapshotResource
}

func (a activeService) GetStackID() string {
	return a.stackID
}

// GetActiveResource returns ActiveResource for swarm.Service in interfaces.ReconcileResource
func (i InitializationService) GetActiveResource(resource interfaces.ReconcileResource) (ActiveResource, error) {
	service, err := i.cli.GetService(resource.ID, interfaces.DefaultGetServiceArg2)
	if err != nil {
		return activeService{}, err
	}
	return i.wrapService(service), nil
}

func (i InitializationService) getSnapshotResourceNames(snapshot interfaces.SnapshotStack) []string {
	result := make([]string, 0, len(snapshot.Services))
	for _, snapshotResource := range snapshot.Services {
		result = append(result, snapshotResource.Name)
	}
	return result
}

func (i InitializationService) wrapService(service swarm.Service) ActiveResource {
	stackID, ok := service.Spec.Annotations.Labels[types.StackLabel]
	if !ok {
		stackID = ""
	}
	return activeService{
		SnapshotResource: interfaces.SnapshotResource{
			ID:   service.ID,
			Meta: service.Meta,
			Name: service.Spec.Name,
		},
		service: service,
		stackID: stackID,
	}
}

// GetKind returns interfaces.ReconcileService
func (i InitializationService) GetKind() interfaces.ReconcileKind {
	return interfaces.ReconcileService
}

// CreatePlugin creates algorithmService
func (i InitializationService) CreatePlugin(snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) AlgorithmPlugin {
	return newAlgorithmPluginService(i, snapshot, requestedResource)
}

// NewInitializationSupportService creates InitializationService
func NewInitializationSupportService(cli interfaces.BackendClient) InitializationService {
	return InitializationService{
		cli: cli,
	}
}

func newAlgorithmPluginService(initService InitializationService, snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) *algorithmService {
	result := algorithmService{
		InitializationService: initService,
		requestedResource:     requestedResource,
		stackID:               snapshot.ID,
		stackSpec:             snapshot.CurrentSpec,
		goals:                 map[string]*interfaces.ReconcileResource{},
	}

	for _, resource := range snapshot.Services {
		result.goals[resource.Name] = transform(resource, &result)
	}

	return &result
}

func (a *algorithmService) lookupSpecifiedResource(name string) interface{} {
	return a.lookupServiceSpec(name)
}

func (a *algorithmService) getRequestedResource() *interfaces.ReconcileResource {
	return a.requestedResource
}

func (a *algorithmService) reconcile(stack interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	return reconcileResource(stack, a)
}

func (a *algorithmService) lookupServiceSpec(name string) *swarm.ServiceSpec {
	for _, serviceSpec := range a.stackSpec.Services {
		if name == serviceSpec.Annotations.Name {
			return &serviceSpec
		}
	}
	return nil
}

func (a *algorithmService) getGoalResources() []*interfaces.ReconcileResource {
	result := make([]*interfaces.ReconcileResource, 0, len(a.goals))
	for _, serviceResource := range a.goals {
		result = append(result, serviceResource)
	}
	return result
}

func (a *algorithmService) getSpecifiedResourceNames() []string {
	result := make([]string, 0, len(a.stackSpec.Services))
	for _, serviceSpec := range a.stackSpec.Services {
		result = append(result, serviceSpec.Annotations.Name)
	}
	return result
}

// GetActiveResources returns ActiveResource array for swarm.Services belonging to the stack
func (a *algorithmService) GetActiveResources() ([]ActiveResource, error) {
	services, err := a.cli.GetServices(dockerTypes.ServiceListOptions{
		Filters: stackLabelFilter(a.stackID),
	})
	if err != nil {
		return []ActiveResource{}, err
	}
	result := make([]ActiveResource, 0, len(services))
	for _, service := range services {
		result = append(result, a.wrapService(service))
	}
	return result, nil
}

func (a *algorithmService) getGoalResource(name string) *interfaces.ReconcileResource {
	for _, serviceResource := range a.goals {
		if name == serviceResource.Name {
			return serviceResource
		}
	}
	return nil
}

func (a *algorithmService) addCreateResourceGoal(specName string) *interfaces.ReconcileResource {
	// returning nil serviceSpec will generate a panic but that is a bug in the
	// calling code
	serviceSpec := a.lookupServiceSpec(specName)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: serviceSpec.Annotations.Name,
		},
		Config: serviceSpec,
		Kind:   a.GetKind(),
	}
	a.goals[specName] = resource
	return resource
}

func (a *algorithmService) addRemoveResourceGoal(activeResource ActiveResource) *interfaces.ReconcileResource {
	activeService := activeResource.(activeService)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: activeService.SnapshotResource,
		Kind:             a.GetKind(),
		Config:           activeService.service.Spec,
	}
	a.goals[activeService.Name] = resource
	return resource
}

func (a *algorithmService) storeGoals(previous interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	goalServices := []interfaces.SnapshotResource{}
	for _, resource := range a.goals {
		if resource.Mark == interfaces.ReconcileDelete {
			continue
		}
		goalServices = append(goalServices, resource.SnapshotResource)
	}

	// Simple copy + override
	updated := previous
	updated.Services = goalServices

	current, err := a.cli.UpdateSnapshotStack(a.stackID,
		updated,
		updated.Meta.Version.Index)
	if err != nil {
		return previous, err
	}

	return current, nil
}

func (a *algorithmService) hasSameConfiguration(resource interfaces.ReconcileResource, actual ActiveResource) bool {
	one := resource.Config.(*swarm.ServiceSpec)
	two := actual.(activeService).service.Spec
	return one.Annotations.Name == two.Annotations.Name &&
		compareMapsIgnoreStackLabel(one.Annotations.Labels, two.Annotations.Labels) &&
		reflect.DeepEqual(one.TaskTemplate, two.TaskTemplate) &&
		reflect.DeepEqual(one.Mode, two.Mode) &&
		reflect.DeepEqual(one.UpdateConfig, two.UpdateConfig) &&
		reflect.DeepEqual(one.RollbackConfig, two.RollbackConfig) &&
		reflect.DeepEqual(one.Networks, two.Networks) &&
		reflect.DeepEqual(one.EndpointSpec, two.EndpointSpec)
}

func (a *algorithmService) CreateResource(resource *interfaces.ReconcileResource) error {
	serviceSpec := resource.Config.(*swarm.ServiceSpec)
	if serviceSpec.Annotations.Labels == nil {
		serviceSpec.Annotations.Labels = map[string]string{}
	}
	serviceSpec.Annotations.Labels[types.StackLabel] = a.stackID
	resp, err := a.cli.CreateService(*serviceSpec,
		interfaces.DefaultCreateServiceArg2,
		interfaces.DefaultCreateServiceArg3)
	if err != nil {
		return err
	}
	resource.ID = resp.ID
	return nil
}

func (a *algorithmService) DeleteResource(resource *interfaces.ReconcileResource) error {
	err := a.cli.RemoveService(resource.ID)
	// Ignore not found error
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	resource.ID = ""
	return nil
}

func (a *algorithmService) UpdateResource(resource interfaces.ReconcileResource) error {
	// the response from UpdateService is irrelevant
	_, err := a.cli.UpdateService(
		resource.ID,
		resource.Meta.Version.Index,
		*resource.Config.(*swarm.ServiceSpec),
		interfaces.DefaultUpdateServiceArg4,
		interfaces.DefaultUpdateServiceArg5)
	if err != nil {
		return err
	}
	return nil
}
