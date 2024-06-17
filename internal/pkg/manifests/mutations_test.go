package manifests

import (
	"testing"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func TestGetMutateFunc_MutateObjectMeta(t *testing.T) {
	want := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"test": "test",
			},
			Annotations: map[string]string{
				"test": "test",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "thanos.io/v1alpha1",
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
					Kind:               "ThanosX",
					Name:               "thanos-testing",
					UID:                "6128aa83-de7f-47c0-abf2-4a380713b599",
				},
			},
		},
	}

	got := &corev1.ConfigMap{}
	f := MutateFuncFor(got, want, nil)
	err := f()
	require.NoError(t, err)

	// Partial mutation checks
	require.Exactly(t, got.Labels, want.Labels)
	require.Exactly(t, got.Annotations, want.Annotations)
	require.Exactly(t, got.OwnerReferences, want.OwnerReferences)
}

func TestGetMutateFunc_ReturnErrOnNotSupportedType(t *testing.T) {
	got := &corev1.Endpoints{}
	want := &corev1.Endpoints{}
	f := MutateFuncFor(got, want, nil)

	require.Error(t, f())
}

func TestGetMutateFunc_MutateConfigMap(t *testing.T) {
	got := &corev1.ConfigMap{
		Data:       map[string]string{"test": "remain"},
		BinaryData: map[string][]byte{},
	}

	want := &corev1.ConfigMap{
		Data:       map[string]string{"test": "test"},
		BinaryData: map[string][]byte{"btest": []byte("btestss")},
	}

	f := MutateFuncFor(got, want, nil)
	err := f()
	require.NoError(t, err)

	// Ensure partial mutation applied
	require.Equal(t, got.Labels, want.Labels)
	require.Equal(t, got.Annotations, want.Annotations)
	require.Equal(t, got.BinaryData, got.BinaryData)
	require.Equal(t, got.Data, want.Data)
}

func TestGetMutateFunc_MutateServiceSpec(t *testing.T) {
	got := &corev1.Service{
		Spec: corev1.ServiceSpec{
			ClusterIP:  "none",
			ClusterIPs: []string{"8.8.8.8"},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       7777,
					TargetPort: intstr.FromString("8888"),
				},
			},
			Selector: map[string]string{
				"select": "that",
			},
		},
	}

	want := &corev1.Service{
		Spec: corev1.ServiceSpec{
			ClusterIP:  "none",
			ClusterIPs: []string{"8.8.8.8", "9.9.9.9"},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       9999,
					TargetPort: intstr.FromString("1111"),
				},
			},
			Selector: map[string]string{
				"select": "that",
				"and":    "other",
			},
		},
	}

	f := MutateFuncFor(got, want, nil)
	err := f()
	require.NoError(t, err)

	// Ensure partial mutation applied
	require.ElementsMatch(t, got.Spec.Ports, want.Spec.Ports)
	require.Exactly(t, got.Spec.Selector, want.Spec.Selector)

	// Ensure not mutated
	require.Equal(t, got.Spec.ClusterIP, "none")
	require.Exactly(t, got.Spec.ClusterIPs, []string{"8.8.8.8"})
}

func TestGetMutateFunc_MutateServiceAccountObjectMeta(t *testing.T) {
	type test struct {
		name string
		got  *corev1.ServiceAccount
		want *corev1.ServiceAccount
	}
	table := []test{
		{
			name: "update object meta",
			got: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
					Annotations: map[string]string{
						"test": "test",
					},
				},
			},
			want: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test":  "test",
						"other": "label",
					},
					Annotations: map[string]string{
						"test":  "test",
						"other": "annotation",
					},
				},
			},
		},
		{
			name: "no update secrets",
			got: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
					Annotations: map[string]string{
						"test": "test",
					},
				},
				Secrets: []corev1.ObjectReference{
					{Name: "secret-me"},
				},
			},
			want: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test":  "test",
						"other": "label",
					},
					Annotations: map[string]string{
						"test":  "test",
						"other": "annotation",
					},
				},
				Secrets: []corev1.ObjectReference{
					{Name: "secret-me"},
					{Name: "another-secret"},
				},
			},
		},
		{
			name: "no update image pull secrets",
			got: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
					Annotations: map[string]string{
						"test": "test",
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "secret-me"},
				},
			},
			want: &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test":  "test",
						"other": "label",
					},
					Annotations: map[string]string{
						"test":  "test",
						"other": "annotation",
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "secret-me"},
					{Name: "another-secret"},
				},
			},
		},
	}

	for _, tt := range table {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := MutateFuncFor(tt.got, tt.want, nil)
			err := f()
			require.NoError(t, err)

			// Partial mutation checks
			require.Exactly(t, tt.got.Labels, tt.want.Labels)
			require.Exactly(t, tt.got.Annotations, tt.want.Annotations)

			if tt.got.Secrets != nil {
				require.NotEqual(t, tt.got.Secrets, tt.want.Secrets)
			}
			if tt.got.ImagePullSecrets != nil {
				require.NotEqual(t, tt.got.ImagePullSecrets, tt.want.ImagePullSecrets)
			}
		})
	}
}

func TestMutateFuncFor_MutateDeploymentSpec(t *testing.T) {
	type test struct {
		name string
		got  *appsv1.Deployment
		want *appsv1.Deployment
	}
	table := []test{
		{
			name: "initial creation",
			got: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
						},
					},
					Replicas: ptr.To[int32](1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test"},
							},
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RecreateDeploymentStrategyType,
					},
				},
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
							"and":  "another",
						},
					},
					Replicas: ptr.To[int32](2),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "test",
									Args: []string{"--do-nothing"},
								},
							},
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RollingUpdateDeploymentStrategyType,
					},
				},
			},
		},
		{
			name: "update spec without selector",
			got: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
						},
					},
					Replicas: ptr.To[int32](1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test"},
							},
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RecreateDeploymentStrategyType,
					},
				},
			},
			want: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
							"and":  "another",
						},
					},
					Replicas: ptr.To[int32](2),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "test",
									Args: []string{"--do-nothing"},
								},
							},
						},
					},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RollingUpdateDeploymentStrategyType,
					},
				},
			},
		},
		{
			name: "remove extra annotations and labels on pod",
			got: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"first-key":  "first-value",
								"second-key": "second-value",
							},
							Labels: map[string]string{
								"first-key":  "first-value",
								"second-key": "second-value",
							},
						},
					},
				},
			},
			want: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"first-key": "first-value",
							},
							Labels: map[string]string{
								"first-key": "first-value",
							},
						},
					},
				},
			},
		},
	}
	for _, tst := range table {
		tst := tst
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			f := MutateFuncFor(tst.got, tst.want, nil)
			err := f()
			require.NoError(t, err)

			// Ensure conditional mutation applied
			if tst.got.CreationTimestamp.IsZero() {
				require.Equal(t, tst.got.Spec.Selector, tst.want.Spec.Selector)
			} else {
				require.NotEqual(t, tst.got.Spec.Selector, tst.want.Spec.Selector)
			}

			// Ensure partial mutation applied
			require.Equal(t, tst.got.Spec.Replicas, tst.want.Spec.Replicas)
			require.Equal(t, tst.got.Spec.Template, tst.want.Spec.Template)
			require.Equal(t, tst.got.Spec.Strategy, tst.want.Spec.Strategy)
		})
	}
}

func TestMutateFuncFor_MutateStatefulSetSpec(t *testing.T) {
	type test struct {
		name string
		got  *appsv1.StatefulSet
		want *appsv1.StatefulSet
	}
	table := []test{
		{
			name: "initial creation",
			got: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy: appsv1.ParallelPodManagement,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
						},
					},
					Replicas: ptr.To[int32](1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test"},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy: appsv1.OrderedReadyPodManagement,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
							"and":  "another",
						},
					},
					Replicas: ptr.To[int32](2),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "test",
									Args: []string{"--do-nothing"},
								},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
									corev1.ReadOnlyMany,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "update spec without selector",
			got: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()},
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy: appsv1.ParallelPodManagement,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
						},
					},
					Replicas: ptr.To[int32](1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test"},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
								},
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Now()},
				Spec: appsv1.StatefulSetSpec{
					PodManagementPolicy: appsv1.OrderedReadyPodManagement,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test": "test",
							"and":  "another",
						},
					},
					Replicas: ptr.To[int32](2),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "test",
									Args: []string{"--do-nothing"},
								},
							},
						},
					},
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
						{
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									corev1.ReadWriteOnce,
									corev1.ReadWriteMany,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "remove extra annotations and labels on pod",
			got: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"first-key":  "first-value",
								"second-key": "second-value",
							},
							Labels: map[string]string{
								"first-key":  "first-value",
								"second-key": "second-value",
							},
						},
					},
				},
			},
			want: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"first-key": "first-value",
							},
							Labels: map[string]string{
								"first-key": "first-value",
							},
						},
					},
				},
			},
		},
	}
	for _, tst := range table {
		tst := tst
		t.Run(tst.name, func(t *testing.T) {
			t.Parallel()
			f := MutateFuncFor(tst.got, tst.want, nil)
			err := f()
			require.NoError(t, err)

			// Ensure conditional mutation applied
			if tst.got.CreationTimestamp.IsZero() {
				require.Equal(t, tst.got.Spec.Selector, tst.want.Spec.Selector)
			} else {
				require.NotEqual(t, tst.got.Spec.Selector, tst.want.Spec.Selector)
			}

			// Ensure partial mutation applied
			require.Equal(t, tst.got.Spec.Replicas, tst.want.Spec.Replicas)
			require.Equal(t, tst.got.Spec.Template, tst.want.Spec.Template)
			require.Equal(t, tst.got.Spec.VolumeClaimTemplates, tst.got.Spec.VolumeClaimTemplates)
		})
	}
}
