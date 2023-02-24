package controller

import (
	"context"
	"fmt"
	"reflect"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	clusterv1alpha1 "kubesphere.io/api/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/kubefed/pkg/kubefedctl"
)

const (
	kubefedNamespace = "kube-federation-system"
	// Actually host cluster name can be anything, there is only necessary when calling JoinFederation function
	hostClusterName = "kubesphere"
)

type ClusterReconciler struct {
	client.Client

	hostConfig *rest.Config
}

func NewClusterReconciler(config *rest.Config) *ClusterReconciler {
	return &ClusterReconciler{
		hostConfig: config,
	}
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &clusterv1alpha1.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.Infof("syncing cluster %s", cluster.Name)

	// currently we didn't set cluster.Spec.Enable when creating cluster at client side, so only check
	// if we enable cluster.Spec.JoinFederation now
	if !cluster.Spec.JoinFederation {
		klog.V(5).Infof("Skipping to join cluster %s cause it is not expected to join", cluster.Name)
		return ctrl.Result{}, nil
	}

	if len(cluster.Spec.Connection.KubeConfig) == 0 {
		klog.V(5).Infof("Skipping to join cluster %s cause the kubeconfig is empty", cluster.Name)
		return ctrl.Result{}, nil
	}

	// fedCluster := &fedv1b1.KubeFedCluster{}
	// if err := r.Get(ctx, client.ObjectKey{Namespace: kubefedNamespace, Name: req.Name}, fedCluster); err == nil {
	// 	return ctrl.Result{}, nil
	// }

	clusterConfig, err := clientcmd.RESTConfigFromKubeConfig(cluster.Spec.Connection.KubeConfig)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster config for %s: %s", cluster.Name, err)
	}

	if cluster.DeletionTimestamp != nil {
		return ctrl.Result{}, kubefedctl.UnjoinCluster(
			r.hostConfig,
			clusterConfig,
			kubefedNamespace,
			hostClusterName,
			cluster.Name,
			cluster.Name,
			true,
			false,
		)
	}

	if _, err = kubefedctl.JoinCluster(
		r.hostConfig,
		clusterConfig,
		kubefedNamespace,
		hostClusterName,
		cluster.Name,
		fmt.Sprintf("%s-secret", cluster.Name),
		apiextv1.ClusterScoped,
		false,
		false,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) InjectClient(c client.Client) error {
	r.Client = c
	return nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return builder.
		ControllerManagedBy(mgr).
		For(
			&clusterv1alpha1.Cluster{},
			builder.WithPredicates(
				predicate.ResourceVersionChangedPredicate{},
				specChangedPredicate{},
			),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}

type specChangedPredicate struct {
	predicate.Funcs
}

func (specChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	oldObj := e.ObjectOld.(*clusterv1alpha1.Cluster)
	newObj := e.ObjectNew.(*clusterv1alpha1.Cluster)
	return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
}
