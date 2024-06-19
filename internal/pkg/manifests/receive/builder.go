package receive

import (
	"encoding/json"
	"fmt"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	"github.com/prometheus/prometheus/model/labels"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the Thanos Receive component.
	Name = "thanos-receive"

	// RouterComponentName is the name of the Thanos Receive router component.
	RouterComponentName = "thanos-receive-router"
	// RouterHTTPPortName is the name of the HTTP port for the Thanos Receive router component.
	RouterHTTPPortName = "http"
	// RouterHTTPPort is the port number for the HTTP port for the Thanos Receive router component.
	RouterHTTPPort = 10902

	// IngestComponentName is the name of the Thanos Receive ingester component.
	IngestComponentName = "thanos-receive-ingester"
	// IngestGRPCPortName is the name of the gRPC port for the Thanos Receive ingester component.
	IngestGRPCPortName = "grpc"
	// IngestGRPCPort is the port number for the gRPC port for the Thanos Receive ingester component.
	IngestGRPCPort = 10901
	// IngestHTTPPortName is the name of the HTTP port for the Thanos Receive ingester component.
	IngestHTTPPortName = "http"
	// IngestHTTPPort is the port number for the HTTP port for the Thanos Receive ingester component.
	IngestHTTPPort = 10902
	// IngestRemoteWritePortName is the name of the remote write port for the Thanos Receive ingester component.
	IngestRemoteWritePortName = "remote-write"
	// IngestRemoteWritePort is the port number for the remote write port for the Thanos Receive ingester component.
	IngestRemoteWritePort = 19291

	// HashringConfigKey is the key in the ConfigMap for the hashring configuration.
	HashringConfigKey = "hashrings.json"
)

// IngesterOptions for Thanos Receive components
type IngesterOptions struct {
	manifests.Options
	StorageSize    resource.Quantity
	ObjStoreSecret corev1.SecretKeySelector
	Retention      string
}

// RouterOptions for Thanos Receive router
type RouterOptions struct {
	manifests.Options
}

// HashringOptions for Thanos Receive hashring
type HashringOptions struct {
	manifests.Options
	HashringSettings map[string]HashringMeta
}

// HashringMeta represents the metadata for a hashring.
type HashringMeta struct {
	OriginalName             string
	Replicas                 int32
	Tenants                  []string
	TenantMatcherType        TenantMatcher
	AssociatedEndpointSlices discoveryv1.EndpointSliceList
}

// Endpoint represents a single logical member of a hashring.
type Endpoint struct {
	// Address is the address of the endpoint.
	Address string `json:"address"`
	// AZ is the availability zone of the endpoint.
	AZ string `json:"az"`
}

// HashringConfig represents the configuration for a hashring a receiver node knows about.
type HashringConfig struct {
	// Hashring is the name of the hashring.
	Hashring string `json:"hashring,omitempty"`
	// Tenants is a list of tenants that match on this hashring.
	Tenants []string `json:"tenants,omitempty"`
	// TenantMatcherType is the type of tenant matching to use.
	TenantMatcherType TenantMatcher `json:"tenant_matcher_type,omitempty"`
	// Endpoints is a list of endpoints that are part of this hashring.
	Endpoints []Endpoint `json:"endpoints"`
	// Algorithm is the hashing algorithm to use.
	Algorithm HashringAlgorithm `json:"algorithm,omitempty"`
	// ExternalLabels are the external labels to use for this hashring.
	ExternalLabels labels.Labels `json:"external_labels,omitempty"`
}

// BuildIngesters builds the ingesters for Thanos Receive
func BuildIngesters(opts []IngesterOptions) []client.Object {
	var objs []client.Object
	for _, opt := range opts {
		objs = append(objs, BuildIngester(opt)...)
	}
	return objs
}

// BuildIngester builds the ingester for Thanos Receive
func BuildIngester(opts IngesterOptions) []client.Object {
	var objs []client.Object
	objs = append(objs, manifests.BuildServiceAccount(opts.Options))
	objs = append(objs, NewIngestorService(opts))
	objs = append(objs, NewIngestorStatefulSet(opts))
	return objs
}

// UnmarshalJSON unmarshals the endpoint from JSON.
func (e *Endpoint) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string.
	err := json.Unmarshal(data, &e.Address)
	if err == nil {
		return nil
	}

	// If that fails, try to unmarshal as an endpoint object.
	type endpointAlias Endpoint
	var configEndpoint endpointAlias
	err = json.Unmarshal(data, &configEndpoint)
	if err == nil {
		e.Address = configEndpoint.Address
		e.AZ = configEndpoint.AZ
	}
	return err
}

// TenantMatcher represents the type of tenant matching to use.
type TenantMatcher string

const (
	// TenantMatcherTypeExact matches tenants exactly. This is also the default one.
	TenantMatcherTypeExact TenantMatcher = "exact"
	// TenantMatcherGlob matches tenants using glob patterns.
	TenantMatcherGlob TenantMatcher = "glob"
)

// HashringAlgorithm represents the hashing algorithm to use.
type HashringAlgorithm string

const (
	// AlgorithmKetama is the ketama hashing algorithm.
	AlgorithmKetama HashringAlgorithm = "ketama"
)

const (
	ingestObjectStoreEnvVarName = "OBJSTORE_CONFIG"

	dataVolumeName      = "data"
	dataVolumeMountPath = "var/thanos/receive"
)

func NewIngestorStatefulSet(opts IngesterOptions) *appsv1.StatefulSet {
	defaultLabels := labelsForIngestor(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)

	vc := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataVolumeName,
				Namespace: opts.Namespace,
				Labels:    aggregatedLabels,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: opts.StorageSize,
					},
				},
			},
		},
	}

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: opts.Name,
			Replicas:    ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels,
			},
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: aggregatedLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:           opts.GetContainerImage(),
							Name:            IngestComponentName,
							ImagePullPolicy: corev1.PullAlways,
							// Ensure restrictive context for the container
							// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
								RunAsUser:                ptr.To(int64(10001)),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/ready",
										Port: intstr.FromInt32(IngestHTTPPort),
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    8,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt32(IngestHTTPPort),
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    8,
							},
							Env: []corev1.EnvVar{
								{
									Name: "NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name: ingestObjectStoreEnvVarName,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: opts.ObjStoreSecret.Name,
											},
											Key:      opts.ObjStoreSecret.Key,
											Optional: ptr.To(false),
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      dataVolumeName,
									MountPath: dataVolumeMountPath,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: IngestGRPCPort,
									Name:          IngestGRPCPortName,
								},
								{
									ContainerPort: IngestHTTPPort,
									Name:          IngestHTTPPortName,
								},
								{
									ContainerPort: IngestRemoteWritePort,
									Name:          IngestRemoteWritePortName,
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							Args:                     ingestorArgsFrom(opts),
						},
					},
				},
			},
		},
	}
	return sts
}

func NewIngestorService(opts IngesterOptions) *corev1.Service {
	defaultLabels := labelsForIngestor(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)
	servicePorts := []corev1.ServicePort{
		{
			Name:       IngestGRPCPortName,
			Port:       IngestGRPCPort,
			TargetPort: intstr.FromInt32(IngestGRPCPort),
			Protocol:   "TCP",
		},
		{
			Name:       IngestHTTPPortName,
			Port:       IngestHTTPPort,
			TargetPort: intstr.FromInt32(IngestHTTPPort),
			Protocol:   "TCP",
		},
		{
			Name:       IngestRemoteWritePortName,
			Port:       IngestRemoteWritePort,
			TargetPort: intstr.FromInt32(IngestRemoteWritePort),
			Protocol:   "TCP",
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector:  defaultLabels,
			Ports:     servicePorts,
			ClusterIP: corev1.ClusterIPNone,
		},
	}
	return svc
}

// IngesterNameFromParent returns a name for the ingester based on the parent and the ingester name.
// If the resulting name is longer than allowed, the ingester name is used as a fallback.
func IngesterNameFromParent(receiveName, ingesterName string) string {
	name := fmt.Sprintf("%s-%s", receiveName, ingesterName)
	// check if the name is a valid DNS-1123 subdomain
	if len(validation.IsDNS1123Subdomain(name)) == 0 {
		return name
	}
	// fallback to ingester name
	return ingesterName
}

func ingestorArgsFrom(opts IngesterOptions) []string {
	args := []string{
		"receive",
		fmt.Sprintf("--log.level=%s", opts.LogLevel),
		fmt.Sprintf("--log.format=%s", opts.LogFormat),
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", IngestGRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", IngestHTTPPort),
		fmt.Sprintf("--remote-write.address=0.0.0.0:%d", IngestRemoteWritePort),
		fmt.Sprintf("--tsdb.path=%s", dataVolumeMountPath),
		fmt.Sprintf("--tsdb.retention=%s", opts.Retention),
		`--label=replica="$(NAME)"`,
		`--label=receive="true"`,
		fmt.Sprintf("--objstore.config=$(%s)", ingestObjectStoreEnvVarName),
		fmt.Sprintf("--receive.local-endpoint=$(NAME).%s.$(NAMESPACE).svc.cluster.local:%d",
			opts.Name, IngestGRPCPort),
		"--receive.grpc-compression=none",
	}
	return args
}

func labelsForIngestor(opts IngesterOptions) map[string]string {
	return map[string]string{
		manifests.NameLabel:            Name,
		manifests.ComponentLabel:       IngestComponentName,
		manifests.InstanceLabel:        opts.Name,
		manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel:       manifests.DefaultManagedByLabel,
		manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
	}
}
