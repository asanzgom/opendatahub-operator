package e2e_test

import (
	"encoding/json"
	"fmt"
	"testing"

	gTypes "github.com/onsi/gomega/types"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/stretchr/testify/require"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/opendatahub-io/opendatahub-operator/v2/api/infrastructure/v1"
	modelregistryctrl "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/modelregistry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"

	. "github.com/onsi/gomega"
)

const (
	testNamespace               = "test-model-registries"   // Namespace used for model registry testing
	dsciInstanceNameDuplicate   = "e2e-test-dsci-duplicate" // Instance name for the duplicate DSCInitialization resource
	dscInstanceNameDuplicate    = "e2e-test-dsc-duplicate"  // Instance name for the duplicate DataScienceCluster resource
	openshiftOperatorsNamespace = "openshift-operators"     // Namespace for OpenShift Operators
	serverlessOperatorNamespace = "openshift-serverless"    // Namespace for the Serverless Operator
)

// DSCTestCtx holds the context for the DSCInitialization and DataScienceCluster management tests.
type DSCTestCtx struct {
	*TestContext
}

// dscManagementTestSuite runs the DataScienceCluster and DSCInitialization management test suite.
func dscManagementTestSuite(t *testing.T) {
	t.Helper()

	// Initialize the test context.
	tc, err := NewTestContext(t)
	require.NoError(t, err, "Failed to initialize test context")

	// Create an instance of test context.
	dscTestCtx := DSCTestCtx{
		TestContext: tc,
	}

	// Define test cases.
	testCases := []TestCase{
		{"Ensure Service Mesh , Serverless and Observability operators are installed", dscTestCtx.ValidateOperatorsInstallation},
		{"Validate creation of DSCInitialization instance", dscTestCtx.ValidateDSCICreation},
		{"Validate creation of DataScienceCluster instance", dscTestCtx.ValidateDSCCreation},
		{"Validate ServiceMeshSpec in DSCInitialization instance", dscTestCtx.ValidateServiceMeshSpecInDSCI},
		//TODO: disabled until RHOAIENG-29225 is resolved
		// {"Validate ServiceMeshControlPlane exists and is recreated upon deletion.", dscTestCtx.ValidateServiceMeshControlPlane},
		{"Validate Knative resource", dscTestCtx.ValidateKnativeSpecInDSC},
		{"Validate owned namespaces exist", dscTestCtx.ValidateOwnedNamespacesAllExist},
		{"Validate default NetworkPolicy exist", dscTestCtx.ValidateDefaultNetworkPolicyExists},
		{"Validate Observability operators are installed", dscTestCtx.ValidateObservabilityOperatorsInstallation},
	}

	// Append webhook-specific tests.
	if testOpts.webhookTest {
		webhookTests := []TestCase{
			{"Validate creation of more than one DSCInitialization instance", dscTestCtx.ValidateDSCIDuplication},
			{"Validate creation of more than one DataScienceCluster instance", dscTestCtx.ValidateDSCDuplication},
			{"Validate Model Registry Configuration Changes", dscTestCtx.ValidateModelRegistryConfig},
		}

		testCases = append(testCases, TestCase{
			name: "Webhook",
			testFn: func(t *testing.T) {
				t.Helper()
				RunTestCases(t, webhookTests)
			},
		})
	}

	// Run the test suite.
	RunTestCases(t, testCases)
}

// ValidateOperatorsInstallation ensures the Service Mesh and Serverless operators are installed.
func (tc *DSCTestCtx) ValidateOperatorsInstallation(t *testing.T) {
	t.Helper()

	// Define operators to be installed.
	operators := []struct {
		nn                types.NamespacedName
		skipOperatorGroup bool
	}{
		{nn: types.NamespacedName{Name: serviceMeshOpName, Namespace: openshiftOperatorsNamespace}, skipOperatorGroup: true},
		{nn: types.NamespacedName{Name: serverlessOpName, Namespace: serverlessOperatorNamespace}, skipOperatorGroup: false},
		{nn: types.NamespacedName{Name: authorinoOpName, Namespace: openshiftOperatorsNamespace}, skipOperatorGroup: true},
		{nn: types.NamespacedName{Name: observabilityOpName, Namespace: observabilityOpNamespace}, skipOperatorGroup: false},
		{nn: types.NamespacedName{Name: telemetryOpName, Namespace: telemetryOpNamespace}, skipOperatorGroup: false},
		{nn: types.NamespacedName{Name: tempoOpName, Namespace: tempoOpNamespace}, skipOperatorGroup: false},
		{nn: types.NamespacedName{Name: opentelemetryOpName, Namespace: opentelemetryOpNamespace}, skipOperatorGroup: false},
	}

	// Create and run test cases in parallel.
	testCases := make([]TestCase, len(operators))
	for i, op := range operators {
		testCases[i] = TestCase{
			name: fmt.Sprintf("Ensure %s is installed", op.nn.Name),
			testFn: func(t *testing.T) {
				t.Helper()
				tc.EnsureOperatorInstalled(op.nn, op.skipOperatorGroup)
			},
		}
	}

	RunTestCases(t, testCases, WithParallel())
}

func (tc *DSCTestCtx) ValidateObservabilityOperatorsInstallation(t *testing.T) {
	t.Helper()

	// Define operators to be installed.
	operators := []struct {
		nn                types.NamespacedName
		skipOperatorGroup bool
	}{
		{nn: types.NamespacedName{Name: telemetryOpName, Namespace: telemetryOpNamespace}, skipOperatorGroup: false},
		{nn: types.NamespacedName{Name: observabilityOpName, Namespace: observabilityOpNamespace}, skipOperatorGroup: false},
	}

	// Create and run test cases in parallel.
	testCases := make([]TestCase, len(operators))
	for i, op := range operators {
		testCases[i] = TestCase{
			name: fmt.Sprintf("Ensure %s is installed", op.nn.Name),
			testFn: func(t *testing.T) {
				t.Helper()
				tc.EnsureOperatorInstalled(op.nn, op.skipOperatorGroup)
			},
		}
	}
	RunTestCases(t, testCases, WithParallel())
}

// ValidateDSCICreation validates the creation of a DSCInitialization.
func (tc *DSCTestCtx) ValidateDSCICreation(t *testing.T) {
	t.Helper()

	tc.EventuallyResourceCreatedOrUpdated(
		WithObjectToCreate(CreateDSCI(tc.DSCInitializationNamespacedName.Name, tc.AppsNamespace)),
		WithCondition(jq.Match(`.status.phase == "%s"`, status.ConditionTypeReady)),
		WithCustomErrorMsg("Failed to create DSCInitialization resource %s", tc.DSCInitializationNamespacedName.Name),

		// Increase time required to get DSCI created
		WithEventuallyTimeout(tc.TestTimeouts.longEventuallyTimeout),
		WithEventuallyPollingInterval(tc.TestTimeouts.defaultEventuallyPollInterval),
	)
}

// ValidateDSCCreation validates the creation of a DataScienceCluster.
func (tc *DSCTestCtx) ValidateDSCCreation(t *testing.T) {
	t.Helper()

	tc.EventuallyResourceCreatedOrUpdated(
		WithObjectToCreate(CreateDSC(tc.DataScienceClusterNamespacedName.Name)),
		WithCondition(jq.Match(`.status.phase == "%s"`, status.ConditionTypeReady)),
		WithCustomErrorMsg("Failed to create DataScienceCluster resource %s", tc.DataScienceClusterNamespacedName.Name),

		// Increase time required to get DSC created
		WithEventuallyTimeout(tc.TestTimeouts.mediumEventuallyTimeout),
		WithEventuallyPollingInterval(tc.TestTimeouts.defaultEventuallyPollInterval),
	)
}

// ValidateServiceMeshSpecInDSCI validates the ServiceMeshSpec within a DSCInitialization instance.
func (tc *DSCTestCtx) ValidateServiceMeshSpecInDSCI(t *testing.T) {
	t.Helper()

	// expected ServiceMeshSpec.
	expServiceMeshSpec := &infrav1.ServiceMeshSpec{
		ManagementState: operatorv1.Managed,
		ControlPlane: infrav1.ControlPlaneSpec{
			Name:              serviceMeshControlPlane,
			Namespace:         serviceMeshNamespace,
			MetricsCollection: serviceMeshMetricsCollection,
		},
		Auth: infrav1.AuthSpec{
			Audiences: &[]string{"https://kubernetes.default.svc"},
		},
	}

	// Marshal the expected ServiceMeshSpec to JSON.
	expServiceMeshSpecJSON, err := json.Marshal(expServiceMeshSpec)
	tc.g.Expect(err).ShouldNot(HaveOccurred(), "Error marshaling expected ServiceMeshSpec")

	// Assert that the actual ServiceMeshSpec matches the expected one.
	tc.EnsureResourceExists(
		WithMinimalObject(gvk.DSCInitialization, tc.DSCInitializationNamespacedName),
		WithCondition(jq.Match(`.spec.serviceMesh == %s`, expServiceMeshSpecJSON)),
		WithCustomErrorMsg("Error validating DSCInitialization instance: Service Mesh spec mismatch"),
	)

	tc.EnsureResourceExists(
		WithMinimalObject(gvk.DSCInitialization, tc.DSCInitializationNamespacedName),
		WithCondition(jq.Match(`.status.phase == "Ready"`)))
}

// ValidateServiceMeshControlPlane checks that ServiceMeshControlPlane exists and is recreated upon deletion.
func (tc *DSCTestCtx) ValidateServiceMeshControlPlane(t *testing.T) {
	t.Helper()

	smcp := types.NamespacedName{Name: serviceMeshControlPlane, Namespace: serviceMeshNamespace}

	// Ensure service mesh feature tracker is in phase ready
	tc.EnsureResourceExists(
		WithMinimalObject(gvk.FeatureTracker, types.NamespacedName{Name: "opendatahub-mesh-control-plane-creation"}),
		WithCondition(jq.Match(`.status.phase == "Ready"`)))

	// Check ServiceMeshControlPlane was created.
	tc.EnsureResourceExists(
		WithMinimalObject(gvk.ServiceMeshControlPlane, smcp),
	)

	// Delete it.
	tc.DeleteResource(
		WithMinimalObject(gvk.ServiceMeshControlPlane, smcp),
		WithWaitForDeletion(true),
	)

	// Check eventually got recreated.
	tc.EnsureResourceExistsConsistently(
		WithMinimalObject(gvk.ServiceMeshControlPlane, smcp),
	)
}

// ValidateKnativeSpecInDSC validates that the Kserve serving spec in the DataScienceCluster matches the expected spec.
func (tc *DSCTestCtx) ValidateKnativeSpecInDSC(t *testing.T) {
	t.Helper()

	// expected ServingSpec
	expServingSpec := &infrav1.ServingSpec{
		ManagementState: operatorv1.Managed,
		Name:            knativeServingNamespace,
		IngressGateway: infrav1.GatewaySpec{
			Certificate: infrav1.CertificateSpec{
				Type: infrav1.OpenshiftDefaultIngress,
			},
		},
	}

	// Marshal the expected ServingSpec to JSON
	expServingSpecJSON, err := json.Marshal(expServingSpec)
	tc.g.Expect(err).ShouldNot(HaveOccurred(), "Error marshaling expected ServingSpec")

	// Assert that the actual ServingSpec matches the expected one.
	tc.EnsureResourceExists(
		WithMinimalObject(gvk.DataScienceCluster, tc.DataScienceClusterNamespacedName),
		WithCondition(jq.Match(`.spec.components.kserve.serving == %s`, expServingSpecJSON)),
		WithCustomErrorMsg("Error validating DSCInitialization instance: Knative Serving spec mismatch"),
	)
}

// ValidateOwnedNamespacesAllExist verifies that the owned namespaces exist.
func (tc *DSCTestCtx) ValidateOwnedNamespacesAllExist(t *testing.T) {
	t.Helper()

	// Ensure namespaces with the owned namespace label exist.
	tc.EnsureResourcesExist(
		WithMinimalObject(gvk.Namespace, types.NamespacedName{}),
		WithListOptions(
			&client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(
					k8slabels.Set{labels.ODH.OwnedNamespace: "true"},
				),
			}),
		WithCondition(BeNumerically(">=", ownedNamespaceNumber)),
		WithCustomErrorMsg("Expected at least %d owned namespaces with label '%s'.", ownedNamespaceNumber, labels.ODH.OwnedNamespace),
	)
}

// ValidateDefaultNetworkPolicyExists verifies that the default network policy exists.
func (tc *DSCTestCtx) ValidateDefaultNetworkPolicyExists(t *testing.T) {
	t.Helper()

	dsci := tc.FetchDSCInitialization()

	// Ensure namespaces with the owned namespace label exist.
	tc.EnsureResourcesExist(
		WithMinimalObject(gvk.NetworkPolicy, types.NamespacedName{Namespace: dsci.Spec.ApplicationsNamespace, Name: dsci.Spec.ApplicationsNamespace}),
		WithCustomErrorMsg("Expected the default NetworkPolicy to be created."),
	)
}

// ValidateDSCIDuplication ensures that no duplicate DSCInitialization resource can be created.
func (tc *DSCTestCtx) ValidateDSCIDuplication(t *testing.T) {
	t.Helper()

	dup := CreateDSCI(dsciInstanceNameDuplicate, tc.AppsNamespace)
	tc.EnsureResourceIsUnique(dup)
}

// ValidateDSCDuplication ensures that no duplicate DataScienceCluster resource can be created.
func (tc *DSCTestCtx) ValidateDSCDuplication(t *testing.T) {
	t.Helper()

	dup := CreateDSC(dscInstanceNameDuplicate)
	tc.EnsureResourceIsUnique(dup, "Error validating DataScienceCluster duplication")
}

// ValidateModelRegistryConfig validates the ModelRegistry configuration changes based on ManagementState.
func (tc *DSCTestCtx) ValidateModelRegistryConfig(t *testing.T) {
	t.Helper()

	// Retrieve the DataScienceCluster object.
	dsc := tc.FetchDataScienceCluster()

	// Check if the ModelRegistry is managed.
	if dsc.Spec.Components.ModelRegistry.ManagementState == operatorv1.Managed {
		// Ensure changing registriesNamespace is not allowed and expect failure.
		tc.UpdateRegistriesNamespace(testNamespace, modelregistryctrl.DefaultModelRegistriesNamespace, true)

		// No further checks if it's managed
		return
	}

	// Ensure setting registriesNamespace to a non-default value is allowed.
	// No error is expected, and we check the value of the patch after it's successful.
	tc.UpdateRegistriesNamespace(testNamespace, testNamespace, false)

	// Ensure resetting registriesNamespace to the default value is allowed.
	tc.UpdateRegistriesNamespace(modelregistryctrl.DefaultModelRegistriesNamespace, modelregistryctrl.DefaultModelRegistriesNamespace, false)
}

// UpdateRegistriesNamespace updates the ModelRegistry component's `RegistriesNamespace` field.
func (tc *DSCTestCtx) UpdateRegistriesNamespace(targetNamespace, expectedValue string, shouldFail bool) {
	// Build the condition:
	// If shouldFail, we expect a failure (Not(Succeed())).
	// If should not fail, we expect the registriesNamespace to match the expected value.
	var expectedCondition gTypes.GomegaMatcher
	if shouldFail {
		expectedCondition = Not(Succeed()) // If shouldFail is true, expect failure.
	} else {
		expectedCondition = And(Succeed(), jq.Match(`.spec.components.modelregistry.registriesNamespace == "%s"`, expectedValue))
	}

	// Update the registriesNamespace field.
	tc.EnsureResourceCreatedOrPatched(
		WithMinimalObject(gvk.DataScienceCluster, tc.DataScienceClusterNamespacedName),
		WithMutateFunc(testf.Transform(`.spec.components.modelregistry.registriesNamespace = "%s"`, targetNamespace)),
		WithCondition(expectedCondition),
		WithCustomErrorMsg("Failed to update RegistriesNamespace to %s, expected %s", targetNamespace, expectedValue),
	)
}
