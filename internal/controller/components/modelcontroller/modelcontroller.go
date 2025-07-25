package modelcontroller

import (
	"context"
	"errors"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
)

type componentHandler struct{}

func init() { //nolint:gochecknoinits
	cr.Add(&componentHandler{})
}

func (s *componentHandler) GetName() string {
	return componentApi.ModelControllerComponentName
}

func (s *componentHandler) NewCRObject(dsc *dscv1.DataScienceCluster) common.PlatformObject {
	// extra logic to set the management .spec.component.managementState, to not leave blank {}
	kState := operatorv1.Removed
	if dsc.Spec.Components.Kserve.ManagementState == operatorv1.Managed {
		kState = operatorv1.Managed
	}

	mState := operatorv1.Removed
	if dsc.Spec.Components.ModelMeshServing.ManagementState == operatorv1.Managed {
		mState = operatorv1.Managed
	}

	mrState := operatorv1.Removed
	if dsc.Spec.Components.ModelRegistry.ManagementState == operatorv1.Managed {
		mrState = operatorv1.Managed
	}

	managementState := operatorv1.Removed
	if kState == operatorv1.Managed || mState == operatorv1.Managed {
		managementState = operatorv1.Managed
	}

	return &componentApi.ModelController{
		TypeMeta: metav1.TypeMeta{
			Kind:       componentApi.ModelControllerKind,
			APIVersion: componentApi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: componentApi.ModelControllerInstanceName,
			Annotations: map[string]string{
				annotations.ManagementStateAnnotation: string(managementState),
			},
		},
		Spec: componentApi.ModelControllerSpec{
			ModelMeshServing: &componentApi.ModelControllerMMSpec{
				ManagementState: mState,
				DevFlagsSpec:    dsc.Spec.Components.ModelMeshServing.DevFlagsSpec,
			},
			Kserve: &componentApi.ModelControllerKerveSpec{
				ManagementState: kState,
				DevFlagsSpec:    dsc.Spec.Components.Kserve.DevFlagsSpec,
				NIM:             dsc.Spec.Components.Kserve.NIM,
			},
			ModelRegistry: &componentApi.ModelControllerMRSpec{
				ManagementState: mrState,
			},
		},
	}
}

// Init for set images.
func (s *componentHandler) Init(_ common.Platform) error {
	// Update image parameters
	if err := odhdeploy.ApplyParams(manifestsPath().String(), imageParamMap); err != nil {
		return fmt.Errorf("failed to update images on path %s: %w", manifestsPath(), err)
	}

	return nil
}

func (s *componentHandler) IsEnabled(dsc *dscv1.DataScienceCluster) bool {
	switch {
	case cr.IsComponentEnabled(componentApi.ModelMeshServingComponentName, dsc):
		return true
	case cr.IsComponentEnabled(componentApi.KserveComponentName, dsc):
		return true
	default:
		return false
	}
}

func (s *componentHandler) UpdateDSCStatus(ctx context.Context, rr *types.ReconciliationRequest) (metav1.ConditionStatus, error) {
	cs := metav1.ConditionUnknown

	c := componentApi.ModelController{}
	c.Name = componentApi.ModelControllerInstanceName

	if err := rr.Client.Get(ctx, client.ObjectKeyFromObject(&c), &c); err != nil && !k8serr.IsNotFound(err) {
		return cs, nil
	}

	dsc, ok := rr.Instance.(*dscv1.DataScienceCluster)
	if !ok {
		return cs, errors.New("failed to convert to DataScienceCluster")
	}

	rr.Conditions.MarkFalse(ReadyConditionType)

	if s.IsEnabled(dsc) {
		if rc := conditions.FindStatusCondition(c.GetStatus(), status.ConditionTypeReady); rc != nil {
			rr.Conditions.MarkFrom(ReadyConditionType, *rc)
			cs = rc.Status
		} else {
			cs = metav1.ConditionFalse
		}
	} else {
		rr.Conditions.MarkFalse(
			ReadyConditionType,
			conditions.WithReason(string(operatorv1.Removed)),
			conditions.WithMessage("Component ManagementState is set to %s", operatorv1.Removed),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)
	}

	return cs, nil
}
