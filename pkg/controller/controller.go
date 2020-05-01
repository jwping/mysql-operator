package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/jwping/mysql-operator/pkg/controller/mysqloperator"
)

func AddToManager(m manager.Manager) (reconcile.Reconciler, error) {
	r := mysqloperator.NewReconciler(m)
	if err := mysqloperator.Add(m, r); err != nil {
		return &mysqloperator.ReconcileMysqlOperator{}, err
	}

	return r, nil
}

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
// var AddToManagerFuncs []func(manager.Manager) error

// // AddToManager adds all Controllers to the Manager
// func AddToManager(m manager.Manager) error {
// 	for _, f := range AddToManagerFuncs {
// 		if err := f(m); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
