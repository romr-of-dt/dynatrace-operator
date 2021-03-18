package capability

import (
	"testing"

	dynatracev1alpha1 "github.com/Dynatrace/dynatrace-operator/api/v1alpha1"
	"github.com/Dynatrace/dynatrace-operator/controllers/customproperties"
	"github.com/Dynatrace/dynatrace-operator/controllers/dtpullsecret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testName                 = "test-name"
	testNamespace            = "test-namespace"
	testKey                  = "test-key"
	testValue                = "test-value"
	testUID                  = "test-uid"
	routingStatefulSetSuffix = "-msgrouter"
	testFeature              = "msgrouter"
)

func TestNewStatefulSetBuilder(t *testing.T) {
	stsBuilder := NewStatefulSetProperties(&dynatracev1alpha1.DynaKube{}, &dynatracev1alpha1.CapabilityProperties{},
		testUID, testValue, "", "", "")
	assert.NotNil(t, stsBuilder)
	assert.NotNil(t, stsBuilder.DynaKube)
	assert.NotNil(t, stsBuilder.CapabilityProperties)
	assert.NotNil(t, stsBuilder.customPropertiesHash)
	assert.NotEmpty(t, stsBuilder.customPropertiesHash)
	assert.NotEmpty(t, stsBuilder.kubeSystemUID)
}

func TestStatefulSetBuilder_Build(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties
	sts, err := CreateStatefulSet(NewStatefulSetProperties(instance, capabilityProperties,
		"", "", testFeature, "", ""))

	assert.NoError(t, err)
	assert.NotNil(t, sts)
	assert.Equal(t, instance.Name+routingStatefulSetSuffix, sts.Name)
	assert.Equal(t, instance.Namespace, sts.Namespace)
	assert.Equal(t, map[string]string{
		KeyDynatrace:  ValueActiveGate,
		KeyActiveGate: instance.Name,
		keyFeature:    testFeature,
	}, sts.Labels)
	assert.Equal(t, instance.Spec.RoutingSpec.Replicas, sts.Spec.Replicas)
	assert.Equal(t, appsv1.ParallelPodManagement, sts.Spec.PodManagementPolicy)
	assert.Equal(t, metav1.LabelSelector{
		MatchLabels: BuildLabelsFromInstance(instance),
	}, *sts.Spec.Selector)
	assert.NotEqual(t, corev1.PodTemplateSpec{}, sts.Spec.Template)
	assert.Equal(t, MergeLabels(
		BuildLabels(instance, capabilityProperties),
		map[string]string{keyFeature: testFeature}), sts.Spec.Template.Labels)
	assert.Equal(t, sts.Labels, sts.Spec.Template.Labels)
	assert.NotEqual(t, corev1.PodSpec{}, sts.Spec.Template.Spec)
	assert.Contains(t, sts.Annotations, AnnotationTemplateHash)

	storedHash := sts.Annotations[AnnotationTemplateHash]
	sts.Annotations = map[string]string{}
	hash, err := generateStatefulSetHash(sts)
	assert.NoError(t, err)
	assert.Equal(t, storedHash, hash)

	t.Run(`template has annotations`, func(t *testing.T) {
		sts, _ := CreateStatefulSet(NewStatefulSetProperties(instance, capabilityProperties,
			"", testValue, "", "", ""))
		assert.Equal(t, map[string]string{
			AnnotationImageVersion:    instance.Status.ActiveGate.ImageVersion,
			AnnotationCustomPropsHash: testValue,
		}, sts.Spec.Template.Annotations)
	})
}

func TestStatefulSet_TemplateSpec(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties
	templateSpec := buildTemplateSpec(NewStatefulSetProperties(instance, capabilityProperties,
		"", "", "", "", ""))

	assert.NotEqual(t, corev1.PodSpec{}, templateSpec)
	assert.NotEmpty(t, templateSpec.Containers)
	assert.Equal(t, capabilityProperties.NodeSelector, templateSpec.NodeSelector)
	assert.Equal(t, capabilityProperties.ServiceAccountName, templateSpec.ServiceAccountName)

	assert.NotEmpty(t, templateSpec.Affinity)
	assert.NotEmpty(t, templateSpec.Affinity.NodeAffinity)
	assert.NotEmpty(t, templateSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	assert.NotEmpty(t, templateSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)

	assert.Contains(t, templateSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      kubernetesBetaArch,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{amd64, arm64},
			},
			{
				Key:      kubernetesBetaOS,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{linux},
			},
		}})
	assert.Contains(t, templateSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      kubernetesArch,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{amd64, arm64},
			},
			{
				Key:      kubernetesOS,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{linux},
			},
		}})
	assert.Equal(t, capabilityProperties.Tolerations, templateSpec.Tolerations)
	assert.Empty(t, templateSpec.Volumes)
	assert.NotEmpty(t, templateSpec.ImagePullSecrets)
	assert.Contains(t, templateSpec.ImagePullSecrets, corev1.LocalObjectReference{Name: instance.Name + dtpullsecret.PullSecretSuffix})
}

func TestStatefulSet_Container(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties
	container := buildContainer(NewStatefulSetProperties(instance, capabilityProperties,
		"", "", "", "", ""))

	assert.Equal(t, dynatracev1alpha1.OperatorName, container.Name)
	assert.Equal(t, instance.ActiveGateImage(), container.Image)
	assert.NotEmpty(t, container.Resources)
	assert.Equal(t, corev1.PullAlways, container.ImagePullPolicy)
	assert.NotEmpty(t, container.Env)
	assert.Empty(t, container.Args)
	assert.Empty(t, container.VolumeMounts)
	assert.NotNil(t, container.ReadinessProbe)
	assert.NotNil(t, container.LivenessProbe)
}

func TestStatefulSet_Volumes(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties

	t.Run(`without custom properties`, func(t *testing.T) {
		volumes := buildVolumes(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.Empty(t, volumes)
	})
	t.Run(`custom properties from value string`, func(t *testing.T) {
		capabilityProperties.CustomProperties = &dynatracev1alpha1.DynaKubeValueSource{
			Value: testValue,
		}
		volumes := buildVolumes(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", testFeature, "", ""))
		expectedSecretName := instance.Name + "-msgrouter-" + customproperties.Suffix

		require.NotEmpty(t, volumes)

		customPropertiesVolume := volumes[0]
		assert.Equal(t, customproperties.VolumeName, customPropertiesVolume.Name)
		assert.NotNil(t, customPropertiesVolume.VolumeSource)
		assert.NotNil(t, customPropertiesVolume.VolumeSource.Secret)
		assert.Equal(t, expectedSecretName, customPropertiesVolume.Secret.SecretName)
		assert.Equal(t, []corev1.KeyToPath{
			{Key: customproperties.DataKey, Path: customproperties.DataPath},
		}, customPropertiesVolume.Secret.Items)
	})
	t.Run(`custom properties from valueFrom`, func(t *testing.T) {
		capabilityProperties.CustomProperties = &dynatracev1alpha1.DynaKubeValueSource{
			ValueFrom: testKey,
		}
		volumes := buildVolumes(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))
		expectedSecretName := testKey

		require.NotEmpty(t, volumes)

		customPropertiesVolume := volumes[0]
		assert.Equal(t, customproperties.VolumeName, customPropertiesVolume.Name)
		assert.NotNil(t, customPropertiesVolume.VolumeSource)
		assert.NotNil(t, customPropertiesVolume.VolumeSource.Secret)
		assert.Equal(t, expectedSecretName, customPropertiesVolume.Secret.SecretName)
		assert.Equal(t, []corev1.KeyToPath{
			{Key: customproperties.DataKey, Path: customproperties.DataPath},
		}, customPropertiesVolume.Secret.Items)
	})
}

func TestStatefulSet_Env(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties

	t.Run(`without proxy`, func(t *testing.T) {
		envVars := buildEnvs(NewStatefulSetProperties(instance, capabilityProperties,
			testUID, "", testFeature, "MSGrouter", ""))
		assert.Equal(t, []corev1.EnvVar{
			{Name: DTCapabilities, Value: "MSGrouter"},
			{Name: DTIdSeedNamespace, Value: instance.Namespace},
			{Name: DTIdSeedClusterId, Value: testUID},
			{Name: testKey, Value: testValue},
		}, envVars)
	})
	t.Run(`with proxy from value`, func(t *testing.T) {
		instance.Spec.Proxy = &dynatracev1alpha1.DynaKubeProxy{Value: testValue}
		envVars := buildEnvs(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.Contains(t, envVars, corev1.EnvVar{
			Name:  DTInternalProxy,
			Value: testValue,
		})
	})
	t.Run(`with proxy from value source`, func(t *testing.T) {
		instance.Spec.Proxy = &dynatracev1alpha1.DynaKubeProxy{ValueFrom: testName}
		envVars := buildEnvs(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.NotEmpty(t, envVars)

		for _, envVar := range envVars {
			if envVar.Name == DTInternalProxy {
				assert.Equal(t, ProxyKey, envVar.ValueFrom.SecretKeyRef.Key)
				assert.Equal(t, corev1.LocalObjectReference{Name: testName}, envVar.ValueFrom.SecretKeyRef.LocalObjectReference)
			}
		}
	})
	t.Run(`with networkzone`, func(t *testing.T) {
		instance := buildTestInstance()
		instance.Spec.NetworkZone = testName
		capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties
		envVars := buildEnvs(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.NotEmpty(t, envVars)

		assert.Contains(t, envVars, corev1.EnvVar{
			Name:  DTNetworkZone,
			Value: testName,
		})
	})
	t.Run(`with group`, func(t *testing.T) {
		instance := buildTestInstance()
		instance.Spec.RoutingSpec.Group = testValue
		capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties
		envVars := buildEnvs(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.NotEmpty(t, envVars)

		assert.Contains(t, envVars, corev1.EnvVar{
			Name:  DTGroup,
			Value: testValue,
		})
	})
}

func TestStatefulSet_VolumeMounts(t *testing.T) {
	instance := buildTestInstance()
	capabilityProperties := &instance.Spec.RoutingSpec.CapabilityProperties

	t.Run(`without custom properties`, func(t *testing.T) {
		volumeMounts := buildVolumeMounts(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))
		assert.Empty(t, volumeMounts)
	})
	t.Run(`with custom properties`, func(t *testing.T) {
		capabilityProperties.CustomProperties = &dynatracev1alpha1.DynaKubeValueSource{Value: testValue}
		volumeMounts := buildVolumeMounts(NewStatefulSetProperties(instance, capabilityProperties,
			"", "", "", "", ""))

		assert.NotEmpty(t, volumeMounts)
		assert.Contains(t, volumeMounts, corev1.VolumeMount{
			ReadOnly:  true,
			Name:      customproperties.VolumeName,
			MountPath: customproperties.MountPath,
			SubPath:   customproperties.DataPath,
		})
	})
}

func buildTestInstance() *dynatracev1alpha1.DynaKube {
	replicas := int32(3)

	return &dynatracev1alpha1.DynaKube{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
		},
		Spec: dynatracev1alpha1.DynaKubeSpec{
			RoutingSpec: dynatracev1alpha1.RoutingSpec{
				CapabilityProperties: dynatracev1alpha1.CapabilityProperties{
					Replicas:    &replicas,
					Tolerations: []corev1.Toleration{{Value: testValue}},
					NodeSelector: map[string]string{
						testKey: testValue,
					},
					ServiceAccountName: testName,
					Env: []corev1.EnvVar{
						{Name: testKey, Value: testValue},
					},
					Args: []string{
						testKey,
					},
				}},
		},
	}
}