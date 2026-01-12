package receive

import (
	"fmt"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	// CapnProtoPortName is the name of the Cap'n Proto port for the Thanos Receive components.
	CapnProtoPortName = "capnproto"
	// CapnProtoPort is the port number for the Cap'n Proto port for the Thanos Receive components.
	CapnProtoPort = 19391
	// RemoteWritePortName is the name of the remote write port for the Thanos Receive components.
	RemoteWritePortName = "remote-write"
	// RemoteWritePort is the port number for the remote write port for the Thanos Receive components.
	RemoteWritePort = 19291

	// HashringConfigKey is the key in the ConfigMap for the hashring configuration.
	HashringConfigKey = "hashrings.json"
	// EmptyHashringConfig is the empty hashring configuration.
	EmptyHashringConfig = "[{}]"
)

// IngesterOptions for Thanos Receive components
type IngesterOptions struct {
	manifests.Options
	TSDBOpts
	TenancyOpts
	StoreLimitsOpts manifests.StoreLimitsOpts
	StorageConfig   manifests.StorageConfig
	ObjStoreSecret  corev1.SecretKeySelector
	ExternalLabels  map[string]string
	// HashringName is the name of the hashring and is a required field.
	HashringName             string
	AsyncForwardWorkerCount  string
	TooFarInFutureTimeWindow manifests.Duration
	ReplicationProtocol      string
	GRPCCompression          string
}

type TSDBOpts struct {
	Retention string
}

type TenancyOpts struct {
	TenantHeader           string
	TenantCertificateField string
	DefaultTenantID        string
	SplitTenantLabelName   string
	TenantLabelName        string
}

// RouterOptions for Thanos Receive router
type RouterOptions struct {
	manifests.Options
	ReplicationFactor     int32
	ExternalLabels        map[string]string
	HashringConfig        string
	HashringAlgorithm     string
	ReplicationProtocol   string
	EnableKubeResourceSync bool
}

// Build builds the ingester for Thanos Receive
func (opts IngesterOptions) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetIngesterLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(name, opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newIngestorService(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newIngestorStatefulSet(opts, selectorLabels, objectMetaLabels))

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
	}

	if opts.ServiceMonitorConfig != nil {
		smLabels := manifests.MergeLabels(opts.ServiceMonitorConfig.Labels, objectMetaLabels)
		objs = append(objs, manifests.BuildServiceMonitor(name, opts.Namespace, selectorLabels, smLabels, serviceMonitorOpts(opts.ServiceMonitorConfig)))
	}
	return objs
}

func (opts IngesterOptions) Valid() error {
	if opts.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if opts.HashringName == "" {
		return fmt.Errorf("hashring name cannot be empty")
	}
	return nil
}

func (opts IngesterOptions) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s-%s", IngestComponentName, opts.Owner, opts.HashringName)
	return manifests.ValidateAndSanitizeResourceName(name)
}

// Build builds the Thanos Receive router components
func (opts RouterOptions) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetRouterLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(name, opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newRouterService(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newRouterDeployment(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newHashringConfigMap(name, opts.Namespace, opts.HashringConfig, objectMetaLabels))

	if opts.EnableKubeResourceSync {
		objs = append(objs, newRouterRole(name, opts.Namespace, objectMetaLabels))
		objs = append(objs, newRouterRoleBinding(name, opts.Namespace, objectMetaLabels))
	}

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
	}

	if opts.ServiceMonitorConfig != nil {
		objs = append(objs, manifests.BuildServiceMonitor(name, opts.Namespace, objectMetaLabels, selectorLabels, serviceMonitorOpts(opts.ServiceMonitorConfig)))
	}
	return objs
}

func (opts RouterOptions) Valid() error {
	if opts.Owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	return nil
}

func (opts RouterOptions) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s", RouterComponentName, opts.Owner)
	return manifests.ValidateAndSanitizeResourceName(name)
}

const (
	ingestObjectStoreEnvVarName = "OBJSTORE_CONFIG"

	dataVolumeName      = "data"
	dataVolumeMountPath = "/var/thanos/receive"
)

// NewIngestorStatefulSet creates a new StatefulSet for the Thanos Receive ingester.
func NewIngestorStatefulSet(opts IngesterOptions) *appsv1.StatefulSet {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetIngesterLabels(opts)
	return newIngestorStatefulSet(opts, selectorLabels, objectMetaLabels)
}

func newIngestorStatefulSet(opts IngesterOptions, selectorLabels, objectMetaLabels map[string]string) *appsv1.StatefulSet {
	name := opts.GetGeneratedResourceName()
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
						corev1.ResourceStorage: opts.StorageConfig.StorageSize,
					},
				},
				StorageClassName: opts.StorageConfig.StorageClassName,
			},
		},
	}

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			PodManagementPolicy:  appsv1.PodManagementPolicyType(opts.PodManagementPolicy),
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: name,
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
									ContainerPort: CapnProtoPort,
									Name:          CapnProtoPortName,
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
	selectorLabels := opts.GetSelectorLabels()
	svc := newService(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels), opts.Annotations)
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

func newIngestorService(opts IngesterOptions, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	svc := newService(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations)
	svc.Spec.ClusterIP = corev1.ClusterIPNone

	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

// NewRouterService creates a new Service for the Thanos Receive router.
func NewRouterService(opts RouterOptions) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetRouterLabels(opts)
	return newRouterService(opts, selectorLabels, objectMetaLabels)
}

func newRouterService(opts RouterOptions, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	svc := newService(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations)
	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}
	return svc
}

// newService creates a new Service for the Thanos Receive components.
func newService(name, namespace string, selectorLabels, objectMetaLabels map[string]string, annotations map[string]string) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Name:       GRPCPortName,
			Port:       GRPCPort,
			TargetPort: intstr.FromInt32(GRPCPort),
			Protocol:   "TCP",
		},
		{
			Name:       CapnProtoPortName,
			Port:       CapnProtoPort,
			TargetPort: intstr.FromInt32(CapnProtoPort),
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
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      objectMetaLabels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    servicePorts,
		},
	}
	return svc
}

const (
	hashringVolumeName            = "hashring-config"
	hashringMountPath             = "/var/lib/thanos-receive"
	kubeResourceSyncContainerName = "kube-resource-sync"
)

// NewRouterDeployment creates a new Deployment for the Thanos Receive router.
func NewRouterDeployment(opts RouterOptions) *appsv1.Deployment {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetRouterLabels(opts)
	return newRouterDeployment(opts, selectorLabels, objectMetaLabels)
}

func newRouterDeployment(opts RouterOptions, selectorLabels, objectMetaLabels map[string]string) *appsv1.Deployment {
	name := opts.GetGeneratedResourceName()
	volumes := buildRouterVolumes(opts, name)
	containers := buildRouterContainers(opts)
	initContainers := buildRouterInitContainers(opts)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: opts.Namespace,
					Labels:    objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					SecurityContext:              &corev1.PodSecurityContext{},
					Volumes:                      volumes,
					InitContainers:               initContainers,
					Containers:                   containers,
					ServiceAccountName:           name,
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
		fmt.Sprintf("--receive.local-endpoint=$(POD_NAME).%s.$(POD_NAMESPACE).svc:%d",
			opts.GetGeneratedResourceName(), GRPCPort),
		fmt.Sprintf("--receive.forward.async-workers=%s", opts.AsyncForwardWorkerCount),
		fmt.Sprintf("--tsdb.too-far-in-future.time-window=%s", opts.TooFarInFutureTimeWindow),
		fmt.Sprintf("--receive.tenant-header=%s", opts.TenancyOpts.TenantHeader),
		fmt.Sprintf("--receive.tenant-certificate-field=%s", opts.TenancyOpts.TenantCertificateField),
		fmt.Sprintf("--receive.default-tenant-id=%s", opts.TenancyOpts.DefaultTenantID),
		fmt.Sprintf("--receive.split-tenant-label-name=%s", opts.TenancyOpts.SplitTenantLabelName),
		fmt.Sprintf("--receive.tenant-label-name=%s", opts.TenancyOpts.TenantLabelName),
	)

	args = append(args, opts.StoreLimitsOpts.ToFlags()...)

	for k, v := range opts.ExternalLabels {
		args = append(args, fmt.Sprintf(`--label=%s="%s"`, k, v))
	}

	if opts.ReplicationProtocol == "capnproto" {
		args = append(args, fmt.Sprintf("--receive.capnproto-address=0.0.0.0:%d", CapnProtoPort))
	}

	// Snappy is the default compression algorithm set on Thanos.
	if opts.GRPCCompression == "none" {
		args = append(args, fmt.Sprintf("--receive.grpc-compression=%s", opts.GRPCCompression))
	}

	// TODO(saswatamcode): Add some validation.
	return manifests.PruneEmptyArgs(args)
}

func routerArgsFrom(opts RouterOptions) []string {
	args := []string{"receive"}
	args = append(args, opts.ToFlags()...)

	grpcDisableEndlessRetry := `{
  "loadBalancingPolicy":"round_robin",
  "retryPolicy": {
    "maxAttempts": 2,
    "initialBackoff": "0.1s",
    "backoffMultiplier": 1,
    "retryableStatusCodes": [
  	  "UNAVAILABLE"
    ]
  }
}`

	args = append(args,
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--remote-write.address=0.0.0.0:%d", RemoteWritePort),
		fmt.Sprintf("--receive.replication-factor=%d", opts.ReplicationFactor),
		fmt.Sprintf("--receive.hashrings-algorithm=%s", opts.HashringAlgorithm),
		fmt.Sprintf("--receive.hashrings-file=%s/%s", hashringMountPath, HashringConfigKey),
		fmt.Sprintf("--receive.grpc-service-config=%s", grpcDisableEndlessRetry),
	)
	for k, v := range opts.ExternalLabels {
		args = append(args, fmt.Sprintf(`--label=%s="%s"`, k, v))
	}

	if opts.ReplicationProtocol == "capnproto" {
		args = append(args, fmt.Sprintf("--receive.replication-protocol=%s", opts.ReplicationProtocol))
	}

	return manifests.PruneEmptyArgs(args)
}

// newHashringConfigMap creates a skeleton ConfigMap for the hashring configuration.
func newHashringConfigMap(name, namespace, contents string, objectMetaLabels map[string]string) *corev1.ConfigMap {
	if contents == "" {
		contents = EmptyHashringConfig
	}
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Labels:    objectMetaLabels,
			Namespace: namespace,
		},
		Data: map[string]string{
			HashringConfigKey: contents,
		},
	}
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
// These labels are guaranteed to be present on all ingest resources created by this package.
func GetRequiredIngesterLabels() map[string]string {
	l := GetRequiredLabels()
	l[manifests.ComponentLabel] = IngestComponentName
	return manifests.MergeLabels(l, manifestsstore.GetRequiredStoreServiceLabel())
}

func (opts IngesterOptions) GetSelectorLabels() map[string]string {
	l := GetRequiredIngesterLabels()
	l[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	l[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.Owner)
	if opts.HashringName != "" {
		l[manifests.HashringLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.HashringName)
	}
	return l
}

func GetIngesterLabels(opts IngesterOptions) map[string]string {
	l := opts.GetSelectorLabels()
	return manifests.SanitizeStoreAPIEndpointLabels(manifests.MergeLabels(opts.Labels, l))
}

// GetRequiredRouterLabels returns a map of labels that can be used to look up thanos receive router resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredRouterLabels() map[string]string {
	l := GetRequiredLabels()
	l[manifests.ComponentLabel] = RouterComponentName
	return manifests.SanitizeStoreAPIEndpointLabels(l)
}

func (opts RouterOptions) GetSelectorLabels() map[string]string {
	l := GetRequiredRouterLabels()
	l[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	l[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.Owner)
	return l
}

func GetRouterLabels(opts RouterOptions) map[string]string {
	l := opts.GetSelectorLabels()
	return manifests.MergeLabels(opts.Labels, l)
}

func serviceMonitorOpts(from *manifests.ServiceMonitorConfig) manifests.ServiceMonitorOptions {
	return manifests.ServiceMonitorOptions{
		Port:     ptr.To(HTTPPortName),
		Interval: from.Interval,
	}
}

// buildRouterVolumes builds the volumes for the router pod
func buildRouterVolumes(opts RouterOptions, name string) []corev1.Volume {
	if opts.EnableKubeResourceSync {
		// When KubeResourceSync is enabled, use EmptyDir and let the sidecar sync from ConfigMap
		return []corev1.Volume{
			{
				Name: hashringVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
	}

	// Default behavior: mount ConfigMap directly
	return []corev1.Volume{
		{
			Name: hashringVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
					DefaultMode: ptr.To(int32(420)),
				},
			},
		},
	}
}

// buildRouterContainers builds the containers for the router pod
func buildRouterContainers(opts RouterOptions) []corev1.Container {
	containers := []corev1.Container{buildThanosRouterContainer(opts)}
	
	if opts.EnableKubeResourceSync {
		containers = append(containers, buildKubeResourceSyncContainer(opts))
	}
	
	return containers
}

// buildRouterInitContainers builds the init containers for the router pod
func buildRouterInitContainers(opts RouterOptions) []corev1.Container {
	var initContainers []corev1.Container
	
	if opts.EnableKubeResourceSync {
		initContainers = append(initContainers, buildKubeResourceSyncInitContainer(opts))
	}
	
	return initContainers
}

// buildThanosRouterContainer builds the main Thanos router container
func buildThanosRouterContainer(opts RouterOptions) corev1.Container {
	return corev1.Container{
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
				ContainerPort: CapnProtoPort,
				Name:          CapnProtoPortName,
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
	}
}

// buildKubeResourceSyncContainer builds the kube-resource-sync sidecar container
func buildKubeResourceSyncContainer(opts RouterOptions) corev1.Container {
	return corev1.Container{
		Name:            kubeResourceSyncContainerName,
		Image:           "quay.io/philipgough/kube-resource-sync:main", // Default image
		ImagePullPolicy: corev1.PullAlways,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		Args: []string{
			"-resource-type=configmap",
			"-resource-name=" + opts.GetGeneratedResourceName(),
			"-namespace=" + opts.Namespace,
			"-write-path=" + hashringMountPath + "/" + HashringConfigKey,
			"-resource-key=" + HashringConfigKey,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      hashringVolumeName,
				MountPath: hashringMountPath,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
	}
}

// buildKubeResourceSyncInitContainer builds the kube-resource-sync init container
// to ensure ConfigMap data is available before the main container starts
func buildKubeResourceSyncInitContainer(opts RouterOptions) corev1.Container {
	container := buildKubeResourceSyncContainer(opts)
	container.Name = kubeResourceSyncContainerName + "-init"
	
	// Add the --init-mode flag for init container behavior
	container.Args = append(container.Args, "--init-mode")
	
	return container
}

// newRouterRole creates a Role for the router when KubeResourceSync is enabled
func newRouterRole(name, namespace string, objectMetaLabels map[string]string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    objectMetaLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

// newRouterRoleBinding creates a RoleBinding for the router when KubeResourceSync is enabled
func newRouterRoleBinding(name, namespace string, objectMetaLabels map[string]string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    objectMetaLabels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     name,
			APIGroup: rbacv1.SchemeGroupVersion.Group,
		},
	}
}

