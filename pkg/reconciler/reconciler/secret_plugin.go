package reconciler

import (
	"reflect"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"

	"github.com/docker/stacks/pkg/interfaces"
	"github.com/docker/stacks/pkg/types"
)

type activeSecret struct {
	interfaces.SnapshotResource
	secret  swarm.Secret
	stackID string
}

// InitializationSecret is the InitializationSupport variant for swarm.Secret
type InitializationSecret struct {
	cli interfaces.BackendClient
}

type algorithmSecret struct {
	InitializationSecret
	requestedResource *interfaces.ReconcileResource
	stackID           string
	stackSpec         types.StackSpec
	goals             map[string]*interfaces.ReconcileResource
}

func (a activeSecret) GetSnapshot() interfaces.SnapshotResource {
	return a.SnapshotResource
}

func (a activeSecret) GetStackID() string {
	return a.stackID
}

// GetActiveResource returns ActiveResource for swarm.Secret in interfaces.ReconcileResource
func (i InitializationSecret) GetActiveResource(resource interfaces.ReconcileResource) (ActiveResource, error) {
	secret, err := i.cli.GetSecret(resource.ID)
	if err != nil {
		return activeSecret{}, err
	}
	return i.wrapSecret(secret), nil
}

func (i InitializationSecret) getSnapshotResourceNames(snapshot interfaces.SnapshotStack) []string {
	result := make([]string, 0, len(snapshot.Secrets))
	for _, snapshotResource := range snapshot.Secrets {
		result = append(result, snapshotResource.Name)
	}
	return result
}

func (i InitializationSecret) wrapSecret(secret swarm.Secret) ActiveResource {
	stackID, ok := secret.Spec.Annotations.Labels[types.StackLabel]
	if !ok {
		stackID = ""
	}
	return activeSecret{
		SnapshotResource: interfaces.SnapshotResource{
			ID:   secret.ID,
			Meta: secret.Meta,
			Name: secret.Spec.Name,
		},
		secret:  secret,
		stackID: stackID,
	}
}

// GetKind returns interfaces.ReconcileSecret
func (i InitializationSecret) GetKind() interfaces.ReconcileKind {
	return interfaces.ReconcileSecret
}

// CreatePlugin creates algorithmSecret
func (i InitializationSecret) CreatePlugin(snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) AlgorithmPlugin {
	return newAlgorithmPluginSecret(i, snapshot, requestedResource)
}

// NewInitializationSupportSecret creates InitializationSecret
func NewInitializationSupportSecret(cli interfaces.BackendClient) InitializationSecret {
	return InitializationSecret{
		cli: cli,
	}
}

func newAlgorithmPluginSecret(secretInit InitializationSecret, snapshot interfaces.SnapshotStack, requestedResource *interfaces.ReconcileResource) *algorithmSecret {
	result := algorithmSecret{
		InitializationSecret: secretInit,
		requestedResource:    requestedResource,
		stackID:              snapshot.ID,
		stackSpec:            snapshot.CurrentSpec,
		goals:                map[string]*interfaces.ReconcileResource{},
	}

	for _, resource := range snapshot.Secrets {
		result.goals[resource.Name] = transform(resource, &result)
	}

	return &result
}

func (a *algorithmSecret) lookupSpecifiedResource(name string) interface{} {
	return a.lookupSecretSpec(name)
}

func (a *algorithmSecret) getRequestedResource() *interfaces.ReconcileResource {
	return a.requestedResource
}

func (a *algorithmSecret) reconcile(stack interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	return reconcileResource(stack, a)
}

func (a *algorithmSecret) lookupSecretSpec(name string) *swarm.SecretSpec {
	for _, secretSpec := range a.stackSpec.Secrets {
		if name == secretSpec.Annotations.Name {
			return &secretSpec
		}
	}
	return nil
}

func (a *algorithmSecret) getGoalResources() []*interfaces.ReconcileResource {
	result := make([]*interfaces.ReconcileResource, 0, len(a.goals))
	for _, secretResource := range a.goals {
		result = append(result, secretResource)
	}
	return result
}

func (a *algorithmSecret) getSpecifiedResourceNames() []string {
	result := make([]string, 0, len(a.stackSpec.Secrets))
	for _, secretSpec := range a.stackSpec.Secrets {
		result = append(result, secretSpec.Annotations.Name)
	}
	return result
}

// GetActiveResources returns ActiveResource array for swarm.Secrets belonging to the stack
func (a *algorithmSecret) GetActiveResources() ([]ActiveResource, error) {
	secrets, err := a.cli.GetSecrets(dockerTypes.SecretListOptions{
		Filters: stackLabelFilter(a.stackID),
	})
	if err != nil {
		return []ActiveResource{}, err
	}
	result := make([]ActiveResource, 0, len(secrets))
	for _, secret := range secrets {
		result = append(result, a.wrapSecret(secret))
	}
	return result, nil
}

func (a *algorithmSecret) getGoalResource(name string) *interfaces.ReconcileResource {
	for _, secretResource := range a.goals {
		if name == secretResource.Name {
			return secretResource
		}
	}
	return nil
}

func (a *algorithmSecret) addCreateResourceGoal(specName string) *interfaces.ReconcileResource {
	// returning nil secretSpec will generate a panic but that is a bug in
	// the calling code
	secretSpec := a.lookupSecretSpec(specName)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: interfaces.SnapshotResource{
			Name: secretSpec.Annotations.Name,
		},
		Config: secretSpec,
		Kind:   a.GetKind(),
	}
	a.goals[specName] = resource
	return resource
}

func (a *algorithmSecret) addRemoveResourceGoal(activeResource ActiveResource) *interfaces.ReconcileResource {
	activeSecret := activeResource.(activeSecret)
	resource := &interfaces.ReconcileResource{
		SnapshotResource: activeSecret.SnapshotResource,
		Kind:             a.GetKind(),
		Config:           activeSecret.secret.Spec,
	}
	a.goals[activeSecret.Name] = resource
	return resource
}

func (a *algorithmSecret) storeGoals(previous interfaces.SnapshotStack) (interfaces.SnapshotStack, error) {
	goalSecrets := []interfaces.SnapshotResource{}
	for _, resource := range a.goals {
		if resource.Mark == interfaces.ReconcileDelete {
			continue
		}
		goalSecrets = append(goalSecrets, resource.SnapshotResource)
	}

	// Simple copy + override
	updated := previous
	updated.Secrets = goalSecrets

	current, err := a.cli.UpdateSnapshotStack(a.stackID,
		updated,
		updated.Meta.Version.Index)
	if err != nil {
		return previous, err
	}

	return current, nil
}

func (a *algorithmSecret) hasSameConfiguration(resource interfaces.ReconcileResource, actual ActiveResource) bool {
	one := resource.Config.(*swarm.SecretSpec)
	two := actual.(activeSecret).secret.Spec
	return one.Annotations.Name == two.Annotations.Name &&
		compareMapsIgnoreStackLabel(one.Annotations.Labels, two.Annotations.Labels) &&
		reflect.DeepEqual(one.Data, two.Data) &&
		reflect.DeepEqual(one.Driver, two.Driver) &&
		reflect.DeepEqual(one.Templating, two.Templating)
}

func (a *algorithmSecret) CreateResource(resource *interfaces.ReconcileResource) error {
	secretSpec := resource.Config.(*swarm.SecretSpec)
	if secretSpec.Annotations.Labels == nil {
		secretSpec.Annotations.Labels = map[string]string{}
	}
	secretSpec.Annotations.Labels[types.StackLabel] = a.stackID
	id, err := a.cli.CreateSecret(*secretSpec)
	if err != nil {
		return err
	}
	resource.ID = id
	return nil
}

func (a *algorithmSecret) DeleteResource(resource *interfaces.ReconcileResource) error {
	err := a.cli.RemoveSecret(resource.ID)
	// Ignore not found error
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	resource.ID = ""
	return nil
}

func (a *algorithmSecret) UpdateResource(resource interfaces.ReconcileResource) error {
	// the response from UpdateSecret is irrelevant
	err := a.cli.UpdateSecret(
		resource.ID,
		resource.Meta.Version.Index,
		*resource.Config.(*swarm.SecretSpec))
	if err != nil {
		return err
	}
	return nil
}
