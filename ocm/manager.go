package ocm

import (
	"context"
	"embed"
	"fmt"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
)

var FS embed.FS

const (
	AddonName = "kubeslice-addon"
)

func runManagerController() error {
	kubeConfig, err := restclient.InClusterConfig()
	if err != nil {
		return err
	}
	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		fmt.Errorf("Unable to create addon manager: %v", err)
		return err
	}

	agent, err := addonfactory.NewAgentAddonFactory(AddonName, FS, "manifests").
		WithConfigGVRs(controllerv1alpha1.GroupVersion.WithResource(controllerv1alpha1.ResourceClusterConfigs)).
		WithGetValuesFuncs(getValues(kubeConfig)).
		BuildHelmAgentAddon()
	if err != nil {
		fmt.Errorf("Unable to build agent addon: %v", err)
		return err
	}
	if err := addonMgr.AddAgent(agent); err != nil {
		fmt.Errorf("Unable to add agent to addon manager: %v", err)
		return err
	}
	ctx := context.Background()
	go func() {
		err := addonMgr.Start(ctx)
		if err != nil {
			klog.Fatal(err)
		}
	}()

	<-ctx.Done()
	return nil
}
