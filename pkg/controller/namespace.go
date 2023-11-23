package controller

import (
	"context"
	"slices"
	"strings"

	"github.com/iawia002/lia/kubernetes/object"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/kubefed/pkg/controller/util"
)

const (
	federatedNamespaceLabel = "kubesphere.io/kubefed-host-namespace"
	clusterListAnnotation   = "kubefed.kubesphere.io/clusters"
)

type NamespaceReconciler struct {
	client.Client
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, req.NamespacedName, ns); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !isFederatedNamespace(ns) {
		return ctrl.Result{}, nil
	}

	klog.Infof("syncing FederatedNamespace %s", ns.Name)
	defer klog.Infof("syncing FederatedNamespace %s finished", ns.Name)

	federatedNamespace := &unstructured.Unstructured{}
	federatedNamespace.SetAPIVersion("types.kubefed.io/v1beta1")
	federatedNamespace.SetKind("FederatedNamespace")
	federatedNamespace.SetNamespace(ns.Name)
	federatedNamespace.SetName(ns.Name)
	if err := r.Get(ctx, client.ObjectKeyFromObject(federatedNamespace), federatedNamespace); err != nil {
		return ctrl.Result{}, err
	}

	placement, err := util.UnmarshalGenericPlacement(federatedNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	clusters := placement.ClusterNames()
	slices.Sort(clusters)
	if updated := object.AddAnnotation(ns, clusterListAnnotation, strings.Join(clusters, ",")); !updated {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Update(ctx, ns)
}

func isFederatedNamespace(obj metav1.Object) bool {
	return object.GetLabel(obj, federatedNamespaceLabel) == "true"
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()

	return builder.
		ControllerManagedBy(mgr).
		For(
			&corev1.Namespace{},
			builder.WithPredicates(
				predicate.ResourceVersionChangedPredicate{},
				predicate.Funcs{
					CreateFunc: func(createEvent event.CreateEvent) bool {
						return isFederatedNamespace(createEvent.Object)
					},
					DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
						return false
					},
					UpdateFunc: func(updateEvent event.UpdateEvent) bool {
						return isFederatedNamespace(updateEvent.ObjectNew)
					},
					GenericFunc: func(genericEvent event.GenericEvent) bool {
						return isFederatedNamespace(genericEvent.Object)
					},
				},
			),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 2,
		}).
		Complete(r)
}
