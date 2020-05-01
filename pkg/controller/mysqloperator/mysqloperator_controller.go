package mysqloperator

import (
	"context"
	"encoding/json"
	"reflect"

	opsv1alpha1 "github.com/jwping/mysql-operator/pkg/apis/ops/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_mysqloperator")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// var mysqlmonitor *monitor.MysqlMonitor

// var gcancel context.CancelFunc

// Add creates a new MysqlOperator Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// func Add(mgr manager.Manager) error {
// 	// var ctx context.Context
// 	// ctx, gcancel = context.WithCancel(context.Background())
// 	// mysqlmonitor = monitor.New(mgr.GetClient(), ctx)
// 	return add(mgr, newReconciler(mgr))
// }

// newReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	// var ctx context.Context
	// ctx, gcancel = context.WithCancel(context.Background())
	// return &ReconcileMysqlOperator{client: mgr.GetClient(), scheme: mgr.GetScheme(), ctx: ctx}
	return &ReconcileMysqlOperator{client: mgr.GetClient(), scheme: mgr.GetScheme(), run: &runner{exec: utilexec.New()}}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func Add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mysqloperator-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MysqlOperator
	err = c.Watch(&source.Kind{Type: &opsv1alpha1.MysqlOperator{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MysqlOperator
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &opsv1alpha1.MysqlOperator{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMysqlOperator implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMysqlOperator{}

// ReconcileMysqlOperator reconciles a MysqlOperator object
type ReconcileMysqlOperator struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	run *runner
}

var oldSpec opsv1alpha1.MysqlOperatorSpec

// Reconcile reads that state of the cluster for a MysqlOperator object and makes changes based on the state read
// and what is in the MysqlOperator.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMysqlOperator) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MysqlOperator")

	// Fetch the MysqlOperator instance
	// instance = &opsv1alpha1.MysqlOperator{}

	instance, err := getInstance(r)

	if err != nil {
		// if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: "default"}, instance); err != nil {
		// if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	StatefulSet := &appsv1.StatefulSet{}

	if err := r.client.Get(context.TODO(), request.NamespacedName, StatefulSet); err != nil && errors.IsNotFound(err) {

		Stateful := NewStatefulSet(instance)
		// reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", Stateful.Namespace, "Deployment.Name", Stateful.Name)
		// deploy := resources.NewDeploy(instance)
		if err := r.client.Create(context.TODO(), Stateful); err != nil {
			// reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", Stateful.Namespace, "Deployment.Name", Stateful.Name)
			return reconcile.Result{}, err
		}
		service := NewService(instance)
		if err := r.client.Create(context.TODO(), service); err != nil {
			return reconcile.Result{}, err
		}
		data, _ := json.Marshal(instance.Spec.Mysql)
		if instance.Annotations != nil {
			instance.Annotations["spec"] = string(data)
		} else {
			instance.Annotations = map[string]string{"spec": string(data)}
		}

		if err := r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, nil
		}
		oldSpec = instance.Spec
		return reconcile.Result{}, nil
	}

	if oldSpec.Mysql.Size == 0 {
		oldSpec = instance.Spec
	}

	if !reflect.DeepEqual(instance.Spec, oldSpec) {
		newStatefulSet := NewStatefulSet(instance)
		if err := r.client.Update(context.TODO(), newStatefulSet); err != nil {
			// reqLogger.Error(err, "DaemonSet Update failed")
		}

		newService := NewService(instance)
		if err := r.client.Update(context.TODO(), newService); err != nil {
			// reqLogger.Error(err, "Service Update failed")
		}

		oldSpec = instance.Spec

		return reconcile.Result{}, nil

	}

	// // Define a new Pod object
	// pod := newPodForCR(instance)

	// // Set MysqlOperator instance as the owner and controller
	// if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
	// 	return reconcile.Result{}, err
	// }

	// // Check if this Pod already exists
	// found := &corev1.Pod{}
	// err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	// if err != nil && errors.IsNotFound(err) {
	// 	reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
	// 	err = r.client.Create(context.TODO(), pod)
	// 	if err != nil {
	// 		return reconcile.Result{}, err
	// 	}

	// 	// Pod created successfully - don't requeue
	// 	return reconcile.Result{}, nil
	// } else if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// // Pod already exists - don't requeue
	// reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}
