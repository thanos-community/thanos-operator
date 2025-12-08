package main

import (
	"fmt"
	"os"

	"github.com/bwplotka/mimic"
	"github.com/bwplotka/mimic/encoding"
	"github.com/go-kit/log"
	"github.com/magefile/mage/mg"

	"github.com/thanos-community/thanos-operator/goconfig"
)

type Config mg.Namespace

const configDir = "config"

func (Config) generator(component string) *mimic.Generator {
	gen := &mimic.Generator{}
	gen = gen.With(configDir, component)
	gen.Logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	return gen
}

// Generate generates the config files for the operator.
func (c Config) Generate() error {
	managerGen := c.generator("manager")
	managerGen.Add("manager.yaml", encoding.GhodssYAML(goconfig.ControllerManagerNamespace(), goconfig.ControllerManagerDeployment(false)))

	prometheusGen := c.generator("prometheus")
	prometheusGen.Add("monitor.yaml", encoding.GhodssYAML(goconfig.ControllerManagerServiceMonitor()))

	defaultGen := c.generator("default")
	defaultGen.Add("manager_auth_proxy_patch.yaml", encoding.GhodssYAML(goconfig.ManagerAuthProxyPatch()))

	rbacGen := c.generator("rbac")
	rbacGen.Add("auth_proxy_client_clusterrole.yaml", encoding.GhodssYAML(goconfig.AuthProxyClientClusterRole()))
	rbacGen.Add("auth_proxy_role_binding.yaml", encoding.GhodssYAML(goconfig.AuthProxyClusterRoleBinding()))
	rbacGen.Add("auth_proxy_role.yaml", encoding.GhodssYAML(goconfig.AuthProxyClusterRole()))
	rbacGen.Add("auth_proxy_service.yaml", encoding.GhodssYAML(goconfig.AuthProxyService()))
	rbacGen.Add("leader_election_role.yaml", encoding.GhodssYAML(goconfig.LeaderElectionRole()))
	rbacGen.Add("leader_election_role_binding.yaml", encoding.GhodssYAML(goconfig.LeaderElectionRoleBinding()))
	rbacGen.Add("role.yaml", encoding.GhodssYAML(goconfig.ManagerClusterRole()))
	rbacGen.Add("role_binding.yaml", encoding.GhodssYAML(goconfig.ManagerClusterRoleBinding()))
	rbacGen.Add("service_account.yaml", encoding.GhodssYAML(goconfig.ManagerServiceAccount()))
	for _, crd := range goconfig.CRDList {
		rbacGen.Add(crd.ShortName+"_editor_role.yaml", encoding.GhodssYAML(goconfig.CRDEditorClusterRole(crd)))
		rbacGen.Add(crd.ShortName+"_viewer_role.yaml", encoding.GhodssYAML(goconfig.CRDViewerClusterRole(crd)))
	}

	samplesGen := c.generator("samples")
	for _, crd := range goconfig.CRDList {
		samplesGen.Add(fmt.Sprintf("v1alpha1_%s.yaml", crd.ShortName), encoding.GhodssYAML(goconfig.SampleCR(crd)))
	}

	rbacGen.Generate()
	managerGen.Generate()
	prometheusGen.Generate()
	samplesGen.Generate()
	defaultGen.Generate()

	return nil
}
