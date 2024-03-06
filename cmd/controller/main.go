package main

import (
	"os"

	"github.com/iawia002/lia/kubernetes/client"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/kubefed/pkg/controller/util"

	"github.com/kubesphere-extensions/kubefed/pkg/controller"
	"github.com/kubesphere-extensions/kubefed/pkg/scheme"
)

func main() {
	klog.InitFlags(nil)

	app := &cli.App{
		Name:          "controller",
		Usage:         "The controller-manager of kubefed extension",
		AllowExtFlags: true,
		Action: func(c *cli.Context) error {
			config, err := client.BuildConfigFromFlags("", c.String("kubeconfig"))
			if err != nil {
				return err
			}
			return run(config)
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		klog.Exit(err)
	}
}

func run(config *rest.Config) error {
	mgr, err := manager.New(config, manager.Options{
		LeaderElection:          true,
		LeaderElectionNamespace: util.DefaultKubeFedSystemNamespace,
		LeaderElectionID:        "kubesphere-kubefed-controller-manager-leader-election",
		Scheme:                  scheme.Scheme,
		Logger:                  klog.NewKlogr(),
		HealthProbeBindAddress:  ":8081",
	})
	if err != nil {
		return err
	}

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	if err = (&controller.NamespaceReconciler{}).SetupWithManager(mgr); err != nil {
		return err
	}
	clusterReconciler := controller.NewClusterReconciler(config)
	if err = clusterReconciler.SetupWithManager(mgr); err != nil {
		return err
	}
	// Register timed tasker
	if err = mgr.Add(clusterReconciler); err != nil {
		return err
	}

	return mgr.Start(signals.SetupSignalHandler())
}
