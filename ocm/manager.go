package ocm

import (
	"context"
	"embed"
	"fmt"
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	"github.com/kubeslice/kubeslice-controller/ocm/secretfs"
	cert "gomodules.xyz/cert"
	"gomodules.xyz/cert/certstore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	cu "kmodules.xyz/client-go/client"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"time"
)

//go:embed all:manifests
var FS embed.FS

const (
	AddonName = "kubeslice-addon"

	Duration10Yrs  = 10 * 365 * 24 * time.Hour
	CACertName     = "ca"
	ServerCertName = "tls"

	NSMWebhookServiceName       = "admission-webhook-svc"
	NSMWebhookNamespace         = "kubeslice-nsm-webhook-system"
	NSMWebhookConfigSecretName  = "kubeslice-webhook-config"
	KubesliceWebhookServiceName = "kubeslice-webhook-service"
	KubesliceWebhookNamespace   = "kubeslice-system"
)

var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.Install(scheme)
}

func RunManagerController() error {
	kubeConfig, err := restclient.InClusterConfig()
	if err != nil {
		return err
	}

	resyncPeriod := 1 * time.Hour
	hubManager, err := ctrl.NewManager(kubeConfig, manager.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "",
		LeaderElection:         false,
		NewClient:              cu.NewClient,
		Cache: cache.Options{
			SyncPeriod: &resyncPeriod,
		},
	})
	if err != nil {
		return err
	}

	addonMgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		fmt.Errorf("Unable to create addon manager: %v", err)
		return err
	}

	agentCertSecretFS := secretfs.New(hubManager.GetClient(), types.NamespacedName{
		Name:      NSMWebhookConfigSecretName,
		Namespace: "kubeslice-controller",
	})
	cs := certstore.New(agentCertSecretFS, "", Duration10Yrs)
	if err := hubManager.Add(manager.RunnableFunc(func(ctx context.Context) error {
		err = cs.InitCA()
		if err != nil {
			return err
		}

		_, _, err := cs.GetServerCertPair(ServerCertName, cert.AltNames{
			DNSNames: []string{
				fmt.Sprintf("%s.%s.svc", NSMWebhookServiceName, NSMWebhookNamespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", NSMWebhookServiceName, NSMWebhookNamespace),
				fmt.Sprintf("%s.%s.svc", KubesliceWebhookServiceName, KubesliceWebhookNamespace),
				fmt.Sprintf("%s.%s.svc.cluster.local", KubesliceWebhookServiceName, KubesliceWebhookNamespace),
			},
		})
		return err
	})); err != nil {
		klog.Error(err, "unable to initialize cert store")
		os.Exit(1)
	}

	agent, err := addonfactory.NewAgentAddonFactory(AddonName, FS, "manifests/kubeslice-worker").
		WithConfigGVRs(controllerv1alpha1.GroupVersion.WithResource(controllerv1alpha1.ResourceClusterConfigs)).
		WithScheme(scheme).
		WithGetValuesFuncs(getValues(kubeConfig, cs)).
		BuildHelmAgentAddon()
	if err != nil {
		fmt.Errorf("Unable to build agent addon: %v", err)
		return err
	}
	if err := addonMgr.AddAgent(agent); err != nil {
		fmt.Errorf("Unable to add agent to addon manager: %v", err)
		return err
	}
	go func() {
		if err := addonMgr.Start(context.Background()); err != nil {
			fmt.Printf("OCM manager exited with error: %v\n", err)
		}
	}()
	return hubManager.Start(context.Background())
}
