package modelregistry

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
	return componentApi.ModelRegistryComponentName
}

func (s *componentHandler) GetManagementState(dsc *dscv1.DataScienceCluster) operatorv1.ManagementState {
	if dsc.Spec.Components.ModelRegistry.ManagementState == operatorv1.Managed {
		return operatorv1.Managed
	}
	return operatorv1.Removed
}

func (s *componentHandler) Init(_ common.Platform) error {
	mi := baseManifestInfo(BaseManifestsSourcePath)

	if err := odhdeploy.ApplyParams(mi.String(), imagesMap, extraParamsMap); err != nil {
		return fmt.Errorf("failed to update params on path %s: %w", mi, err)
	}

	return nil
}

func (s *componentHandler) NewCRObject(dsc *dscv1.DataScienceCluster) common.PlatformObject {
	return &componentApi.ModelRegistry{
		TypeMeta: metav1.TypeMeta{
			Kind:       componentApi.ModelRegistryKind,
			APIVersion: componentApi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: componentApi.ModelRegistryInstanceName,
			Annotations: map[string]string{
				annotations.ManagementStateAnnotation: string(s.GetManagementState(dsc)),
			},
		},
		Spec: componentApi.ModelRegistrySpec{
			ModelRegistryCommonSpec: dsc.Spec.Components.ModelRegistry.ModelRegistryCommonSpec,
		},
	}
}

func (s *componentHandler) UpdateDSCStatus(ctx context.Context, rr *types.ReconciliationRequest) (metav1.ConditionStatus, error) {
	cs := metav1.ConditionUnknown

	c := componentApi.ModelRegistry{}
	c.Name = componentApi.ModelRegistryInstanceName

	if err := rr.Client.Get(ctx, client.ObjectKeyFromObject(&c), &c); err != nil && !k8serr.IsNotFound(err) {
		return cs, nil
	}

	dsc, ok := rr.Instance.(*dscv1.DataScienceCluster)
	if !ok {
		return cs, errors.New("failed to convert to DataScienceCluster")
	}

	dsc.Status.InstalledComponents[LegacyComponentName] = false
	dsc.Status.Components.ModelRegistry.ManagementSpec.ManagementState = s.GetManagementState(dsc)
	dsc.Status.Components.ModelRegistry.ModelRegistryCommonStatus = nil

	rr.Conditions.MarkFalse(ReadyConditionType)

	switch s.GetManagementState(dsc) {
	case operatorv1.Managed:
		dsc.Status.InstalledComponents[LegacyComponentName] = true
		dsc.Status.Components.ModelRegistry.ModelRegistryCommonStatus = c.Status.ModelRegistryCommonStatus.DeepCopy()

		if rc := conditions.FindStatusCondition(c.GetStatus(), status.ConditionTypeReady); rc != nil {
			rr.Conditions.MarkFrom(ReadyConditionType, *rc)
			cs = rc.Status
		} else {
			cs = metav1.ConditionFalse
		}

	case operatorv1.Removed:
		rr.Conditions.MarkFalse(
			ReadyConditionType,
			conditions.WithReason(string(operatorv1.Removed)),
			conditions.WithMessage("Component ManagementState is set to %s", operatorv1.Removed),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)

	default:
		return cs, fmt.Errorf("unknown state %s ", s.GetManagementState(dsc))
	}

	return cs, nil
}
