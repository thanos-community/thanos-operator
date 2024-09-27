package receive

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	"github.com/go-logr/logr"
	"github.com/prometheus/prometheus/model/labels"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the Thanos Receive component.
	Name = "thanos-receive"

	// RouterComponentName is the name of the Thanos Receive router component.
	RouterComponentName = "thanos-receive-router"
	// IngestComponentName is the name of the Thanos Receive ingester component.
	IngestComponentName = "thanos-receive-ingester"

	// HTTPPortName is the name of the HTTP port for the Thanos Receive components.
	HTTPPortName = "http"
	// HTTPPort is the port number for the HTTP port for the Thanos Receive components.
	HTTPPort = 10902
	// GRPCPortName is the name of the gRPC port for the Thanos Receive components.
	GRPCPortName = "grpc"
	// GRPCPort is the port number for the gRPC port for the Thanos Receive components.
	GRPCPort = 10901
	// RemoteWritePortName is the name of the remote write port for the Thanos Receive components.
	RemoteWritePortName = "remote-write"
	// RemoteWritePort is the port number for the remote write port for the Thanos Receive components.
	RemoteWritePort = 19291

	// HashringConfigKey is the key in the ConfigMap for the hashring configuration.
	HashringConfigKey = "hashrings.json"
	// EmptyHashringConfig is the empty hashring configuration.
	EmptyHashringConfig = "[{}]"

	ShardLabel = "operator.thanos.io/receive-ingester"
)

// IngesterOptions for Thanos Receive components
type IngesterOptions struct {
	manifests.Options
	TSDBOpts
	StorageSize    resource.Quantity
	ObjStoreSecret corev1.SecretKeySelector
	ExternalLabels map[string]string
	// Instance is the owner of the ingester and if not set, defaults to the name of the object.
	Instance string
}

func GetIngesterServiceName(opts IngesterOptions) string {
	return opts.Name
}

func GetIngesterServiceAccountName(opts IngesterOptions) string {
	return opts.Name
}

type TSDBOpts struct {
	Retention string
}

// RouterOptions for Thanos Receive router
type RouterOptions struct {
	manifests.Options
	ReplicationFactor int32
	ExternalLabels    map[string]string
}

func GetRouterServiceName(opts RouterOptions) string {
	return opts.Name
}

func GetRouterServiceAccountName(opts RouterOptions) string {
	return opts.Name
}

// HashringOptions for Thanos Receive hashring
type HashringOptions struct {
	manifests.Options
	// DesiredReplicationFactor is the desired replication factor for the hashrings.
	DesiredReplicationFactor int32
	// HashringSettings is the configuration for the hashrings.
	// The key should be the name of the Service that the hashring is associated with.
	HashringSettings map[string]HashringMeta
}

// HashringMeta represents the metadata for a hashring.
type HashringMeta struct {
	// OriginalName is the original name of the hashring
	OriginalName string
	// DesiredReplicasReplicas is the desired number of replicas for the hashring
	DesiredReplicasReplicas int32
	// Tenants is a list of tenants that match on this hashring.
	Tenants []string
	// TenantMatcherType is the type of tenant matching to use.
	TenantMatcherType TenantMatcher
	// Priority is the priority of the hashring which is used for sorting.
	// If Priority is the same, the hashring will be sorted by name.
	Priority int
	// AssociatedEndpointSlices is the list of EndpointSlices associated with the hashring.
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

// Hashrings is a list of hashrings.
type Hashrings []HashringConfig

// BuildIngester builds the ingester for Thanos Receive
func BuildIngester(opts IngesterOptions) []client.Object {
	var objs []client.Object
	selectorLabels := GetIngesterSelectorLabels(opts)
	objectMetaLabels := GetIngesterLabels(opts)
	// for receive ingesters we create a single service account per instance
	objs = append(objs, manifests.BuildServiceAccount(GetIngesterServiceAccountName(opts), opts.Namespace, selectorLabels))
	objs = append(objs, newIngestorService(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newIngestorStatefulSet(opts, selectorLabels, objectMetaLabels))
	return objs
}

// ErrHashringsEmpty is returned when one or more hashrings are empty
var ErrHashringsEmpty = fmt.Errorf("one or more hashrings are empty")

// BuildHashrings builds the hashrings for Thanos Receive from the provided configuration.
func BuildHashrings(logger logr.Logger, preExistingState *corev1.ConfigMap, opts HashringOptions) (client.Object, error) {
	var currentState []HashringConfig
	if preExistingState != nil && preExistingState.Data != nil && preExistingState.Data[HashringConfigKey] != "" {
		if err := json.Unmarshal([]byte(preExistingState.Data[HashringConfigKey]), &currentState); err != nil {
			return nil, fmt.Errorf("failed to unmarshal current state from ConfigMap: %w", err)
		}
	}
	routerOpts := RouterOptions{
		Options: opts.Options,
	}
	opts.Labels = manifests.MergeLabels(opts.Labels, GetRouterSelectorLabels(routerOpts))
	cm := newHashringConfigMap(opts)

	var hashrings Hashrings
	// iterate over all the input options and build the hashrings
	for hashringName, hashringMeta := range opts.HashringSettings {

		var readyEndpoints []string
		for _, epSlice := range hashringMeta.AssociatedEndpointSlices.Items {
			// validate ownership of the EndpointSlice
			if !isExpectedOwner(logger, epSlice, hashringName) {
				continue
			}

			readyEndpoints = append(readyEndpoints, extractReadyEndpoints(epSlice, hashringName)...)

		}
		// sort and deduplicate the endpoints in case there are duplicates across multiple EndpointSlices
		slices.Sort(readyEndpoints)
		readyEndpoints = slices.Compact(readyEndpoints)
		// convert to local types
		var endpoints []Endpoint
		for _, ep := range readyEndpoints {
			endpoints = append(endpoints, Endpoint{Address: ep})
		}

		// if this is the first time we have seen this hashring,
		// we want to make sure it is fully ready before we add it to the list of hashrings.
		var found bool
		var currentHashring HashringConfig
		for _, hr := range currentState {
			if hr.Hashring == hashringMeta.OriginalName {
				found = true
				currentHashring = hr
				break
			}
		}

		if !found {
			// if we have never seen this before, we want to ensure readiness, otherwise we wait
			if len(endpoints) < int(hashringMeta.DesiredReplicasReplicas) {
				logger.Info("hashring not ready yet, skipping for now", "hashring",
					hashringMeta.OriginalName, "expected", hashringMeta.DesiredReplicasReplicas, "got", len(endpoints),
				)
				continue
			}
		}

		if len(endpoints) < int(opts.DesiredReplicationFactor) {
			// we have a situation here where the hashring is ready but the replication factor is not met
			// this will cause the router to crash - see https://github.com/thanos-io/thanos/issues/7054
			// to avoid this, we will keep the previous state of the hashring if it exists
			if found {
				hashrings = append(hashrings, currentHashring)
			}

		} else {
			// we just take the pre existing ready state of the hashring i.e dynamic scaling
			// todo - we might want to offer different scaling strategies in the future (static, dynamic, etc)
			hashrings = append(hashrings, HashringConfig{
				Hashring:          hashringMeta.OriginalName,
				Tenants:           hashringMeta.Tenants,
				TenantMatcherType: hashringMeta.TenantMatcherType,
				Endpoints:         endpoints,
				ExternalLabels:    nil,
			})
		}

	}

	if len(hashrings) == 0 {
		cm.Data = map[string]string{
			HashringConfigKey: EmptyHashringConfig,
		}
		return cm, ErrHashringsEmpty
	}

	// sort the hashrings by priority or name
	sort.Slice(hashrings, func(i, j int) bool {
		if opts.HashringSettings[hashrings[i].Hashring].Priority == opts.HashringSettings[hashrings[j].Hashring].Priority {
			return hashrings[i].Hashring > hashrings[j].Hashring
		}
		return opts.HashringSettings[hashrings[i].Hashring].Priority > opts.HashringSettings[hashrings[j].Hashring].Priority
	})

	conf, err := hashrings.toJson()
	if err != nil {
		return nil, err
	}

	cm.Data = map[string]string{
		HashringConfigKey: conf,
	}

	return cm, nil
}

// BuildRouter builds the Thanos Receive router components
func BuildRouter(opts RouterOptions) []client.Object {
	selectorLabels := GetRouterSelectorLabels(opts)
	objectMetaLabels := GetRouterLabels(opts)
	return []client.Object{
		manifests.BuildServiceAccount(GetRouterServiceAccountName(opts), opts.Namespace, selectorLabels),
		newRouterService(opts, selectorLabels, objectMetaLabels),
		newRouterDeployment(opts, selectorLabels, objectMetaLabels),
	}
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

// NewIngestorStatefulSet creates a new StatefulSet for the Thanos Receive ingester.
func NewIngestorStatefulSet(opts IngesterOptions) *appsv1.StatefulSet {
	selectorLabels := GetIngesterSelectorLabels(opts)
	objectMetaLabels := GetIngesterLabels(opts)
	return newIngestorStatefulSet(opts, selectorLabels, objectMetaLabels)
}

func newIngestorStatefulSet(opts IngesterOptions, selectorLabels, objectMetaLabels map[string]string) *appsv1.StatefulSet {
	vc := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataVolumeName,
				Namespace: opts.Namespace,
				Labels:    objectMetaLabels,
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
			Labels:    objectMetaLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: GetIngesterServiceName(opts),
			Replicas:    ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: GetIngesterServiceAccountName(opts),
					SecurityContext:    &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							Image:           opts.GetContainerImage(),
							Name:            IngestComponentName,
							ImagePullPolicy: corev1.PullAlways,
							// Ensure restrictive context for the container
							// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								RunAsNonRoot:             ptr.To(true),
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
										Port: intstr.FromInt32(HTTPPort),
									},
								},
								InitialDelaySeconds: 20,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    15,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt32(HTTPPort),
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
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
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
									ContainerPort: GRPCPort,
									Name:          GRPCPortName,
								},
								{
									ContainerPort: HTTPPort,
									Name:          HTTPPortName,
								},
								{
									ContainerPort: RemoteWritePort,
									Name:          RemoteWritePortName,
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
	manifests.AugmentWithOptions(sts, opts.Options)
	return sts
}

// NewIngestorService creates a new Service for the Thanos Receive ingester.
func NewIngestorService(opts IngesterOptions) *corev1.Service {
	selectorLabels := GetIngesterSelectorLabels(opts)
	svc := newService(opts.Name, opts.Namespace, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels))
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

func newIngestorService(opts IngesterOptions, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	svc := newService(GetIngesterServiceName(opts), opts.Namespace, selectorLabels, objectMetaLabels)
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

// NewRouterService creates a new Service for the Thanos Receive router.
func NewRouterService(opts RouterOptions) *corev1.Service {
	selectorLabels := GetRouterSelectorLabels(opts)
	objectMetaLabels := GetRouterLabels(opts)
	return newRouterService(opts, selectorLabels, objectMetaLabels)
}
func newRouterService(opts RouterOptions, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	svc := newService(GetRouterServiceName(opts), opts.Namespace, selectorLabels, objectMetaLabels)
	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}
	return svc
}

// newService creates a new Service for the Thanos Receive components.
func newService(name, namespace string, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Name:       GRPCPortName,
			Port:       GRPCPort,
			TargetPort: intstr.FromInt32(GRPCPort),
			Protocol:   "TCP",
		},
		{
			Name:       HTTPPortName,
			Port:       HTTPPort,
			TargetPort: intstr.FromInt32(HTTPPort),
			Protocol:   "TCP",
		},
		{
			Name:       RemoteWritePortName,
			Port:       RemoteWritePort,
			TargetPort: intstr.FromInt32(RemoteWritePort),
			Protocol:   "TCP",
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    objectMetaLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    servicePorts,
		},
	}
	return svc
}

const (
	hashringVolumeName = "hashring-config"
	hashringMountPath  = "var/lib/thanos-receive"
)

// NewRouterDeployment creates a new Deployment for the Thanos Receive router.
func NewRouterDeployment(opts RouterOptions) *appsv1.Deployment {
	selectorLabels := GetRouterSelectorLabels(opts)
	objectMetaLabels := GetRouterLabels(opts)
	return newRouterDeployment(opts, selectorLabels, objectMetaLabels)
}

func newRouterDeployment(opts RouterOptions, selectorLabels, objectMetaLabels map[string]string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    objectMetaLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      opts.Name,
					Namespace: opts.Namespace,
					Labels:    objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{},
					Volumes: []corev1.Volume{
						{
							Name: hashringVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: opts.Name,
									},
									DefaultMode: ptr.To(int32(420)),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Image:           opts.GetContainerImage(),
							Name:            RouterComponentName,
							ImagePullPolicy: corev1.PullAlways,
							// Ensure restrictive context for the container
							// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot:             ptr.To(true),
								AllowPrivilegeEscalation: ptr.To(false),
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
										Port: intstr.FromInt32(HTTPPort),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    8,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt32(HTTPPort),
									},
								},
								InitialDelaySeconds: 5,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    8,
							},
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      hashringVolumeName,
									MountPath: hashringMountPath,
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: GRPCPort,
									Name:          GRPCPortName,
								},
								{
									ContainerPort: HTTPPort,
									Name:          HTTPPortName,
								},
								{
									ContainerPort: RemoteWritePort,
									Name:          RemoteWritePortName,
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							Args:                     routerArgsFrom(opts),
						},
					},
					ServiceAccountName:           GetRouterServiceAccountName(opts),
					AutomountServiceAccountToken: ptr.To(true),
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: ptr.To(intstr.FromInt32(1)),
					MaxSurge:       ptr.To(intstr.FromInt32(0)),
				},
			},
			RevisionHistoryLimit: ptr.To(int32(10)),
		},
	}
	manifests.AugmentWithOptions(deployment, opts.Options)
	return deployment
}

func ingestorArgsFrom(opts IngesterOptions) []string {
	args := []string{"receive"}
	args = append(args, opts.ToFlags()...)

	args = append(args,
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--remote-write.address=0.0.0.0:%d", RemoteWritePort),
		fmt.Sprintf("--tsdb.path=%s", dataVolumeMountPath),
		fmt.Sprintf("--tsdb.retention=%s", opts.Retention),
		fmt.Sprintf("--objstore.config=$(%s)", ingestObjectStoreEnvVarName),
		fmt.Sprintf("--receive.local-endpoint=$(POD_NAME).%s.$(POD_NAMESPACE).svc.cluster.local:%d",
			opts.Name, GRPCPort),
		"--receive.grpc-compression=none",
	)

	for k, v := range opts.ExternalLabels {
		args = append(args, fmt.Sprintf(`--label=%s="%s"`, k, v))
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

func routerArgsFrom(opts RouterOptions) []string {
	args := []string{"receive"}
	args = append(args, opts.ToFlags()...)
	args = append(args,
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--remote-write.address=0.0.0.0:%d", RemoteWritePort),
		fmt.Sprintf("--receive.replication-factor=%d", opts.ReplicationFactor),
		fmt.Sprintf("--receive.hashrings-algorithm=%s", AlgorithmKetama),
		fmt.Sprintf("--receive.hashrings-file=%s/%s", hashringMountPath, HashringConfigKey),
	)
	for k, v := range opts.ExternalLabels {
		args = append(args, fmt.Sprintf(`--label=%s="%s"`, k, v))
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

// newHashringConfigMap creates a skeleton ConfigMap for the hashring configuration.
func newHashringConfigMap(opts HashringOptions) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Labels:    opts.Labels,
			Namespace: opts.Namespace,
		},
	}
}

// isExpectedOwner checks if the endpoint slice is owned by the service and if the service is belonged to the hashring.
func isExpectedOwner(logger logr.Logger, epSlice discoveryv1.EndpointSlice, expectedOwner string) bool {
	if len(epSlice.GetOwnerReferences()) != 1 {
		logger.Info("skipping endpoint slice with more than one owner",
			"namespace", epSlice.Namespace, "name", epSlice.Name)
		return false
	}

	owner := epSlice.GetOwnerReferences()[0]
	if owner.Kind != "Service" || owner.Name != expectedOwner {
		logger.Info("skipping endpoint slice where owner ref is not a service or does not match hashring name",
			"namespace", epSlice.Namespace, "name", epSlice.Name)
		return false
	}
	return true
}

func extractReadyEndpoints(epSlice discoveryv1.EndpointSlice, svcName string) []string {
	readyEndpoints := make([]string, 0, len(epSlice.Endpoints))
	for _, ep := range epSlice.Endpoints {
		if ep.Hostname == nil {
			continue
		}
		if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
			continue
		}
		readyEndpoints = append(
			readyEndpoints,
			fmt.Sprintf("%s.%s.%s.svc.cluster.local:%d", *ep.Hostname, svcName, epSlice.GetNamespace(), GRPCPort),
		)
	}
	return readyEndpoints
}

// GetRequiredLabels returns a map of labels that can be used to look up thanos receive resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredLabels() map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}

// GetRequiredIngesterLabels returns a map of labels that can be used to look up thanos receive ingest resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredIngesterLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.ComponentLabel] = IngestComponentName
	labels[manifests.DefaultStoreAPILabel] = manifests.DefaultStoreAPIValue
	return labels
}

func GetIngesterSelectorLabels(opts IngesterOptions) map[string]string {
	labels := GetRequiredIngesterLabels()
	labels[manifests.InstanceLabel] = opts.Instance
	labels[ShardLabel] = opts.Name
	return labels
}

func GetIngesterLabels(opts IngesterOptions) map[string]string {
	labels := GetIngesterSelectorLabels(opts)
	return manifests.MergeLabels(opts.Labels, labels)
}

// GetRequiredRouterLabels returns a map of labels that can be used to look up thanos receive router resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredRouterLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.ComponentLabel] = RouterComponentName
	return labels
}

func GetRouterSelectorLabels(opts RouterOptions) map[string]string {
	labels := GetRequiredRouterLabels()
	labels[manifests.InstanceLabel] = opts.Name
	return labels
}

func GetRouterLabels(opts RouterOptions) map[string]string {
	labels := GetRouterSelectorLabels(opts)
	return manifests.MergeLabels(opts.Labels, labels)
}

func (h Hashrings) toJson() (string, error) {
	b, err := json.MarshalIndent(h, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal hashrings: %w", err)
	}
	return string(b), nil
}
