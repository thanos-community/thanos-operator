package suite

import (
	"os"

	operatorconfig "github.com/thanos-community/thanos-operator/config"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func metricsAuthBindingName(namespace string) string {
	return namespace + "-metrics-auth"
}

func operatorObjects(namespace string, features []string) []client.Object {
	deployment := operatorconfig.ControllerManagerDeployment(
		operatorconfig.WithWatchOwnNamespace(),
		operatorconfig.WithFeatures(features...),
	)
	image := os.Getenv("E2E_IMG")
	if image == "" {
		image = "example.com/thanos-operator:v0.0.1"
	}
	deployment.Spec.Template.Spec.Containers[0].Image = image

	leaderBinding := operatorconfig.LeaderElectionRoleBinding()
	leaderBinding.Subjects[0].Namespace = namespace
	objects := []client.Object{
		operatorconfig.ManagerServiceAccount(),
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "manager-rolebinding"},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     operatorconfig.ManagerRoleName,
			},
			Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: deployment.Spec.Template.Spec.ServiceAccountName, Namespace: namespace}},
		},
		operatorconfig.LeaderElectionRole(),
		leaderBinding,
		deployment,
	}
	for _, obj := range objects {
		obj.SetNamespace(namespace)
	}
	metricsBinding := operatorconfig.MetricsAuthClusterRoleBinding()
	metricsBinding.Name = metricsAuthBindingName(namespace)
	metricsBinding.Subjects[0].Namespace = namespace
	return append(objects, metricsBinding)
}
