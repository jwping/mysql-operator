package mysqloperator

import (
	"fmt"

	opsv1alpha1 "github.com/jwping/mysql-operator/pkg/apis/ops/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewStatefulSet(app *opsv1alpha1.MysqlOperator) *appsv1.StatefulSet {
	labels := map[string]string{"app": app.Name, "mysql": "operator"}
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,

			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, schema.GroupVersionKind{
					Group:   opsv1alpha1.SchemeGroupVersion.Group,
					Version: opsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AppService",
				}),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &app.Spec.Mysql.Size,
			ServiceName: app.Name,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					// HostNetwork: true,
					Containers: newContainers(app),
				},
			},

			Selector: selector,
		},
	}
}

func newContainers(app *opsv1alpha1.MysqlOperator) []corev1.Container {
	containerPorts := []corev1.ContainerPort{}
	for _, svcPort := range app.Spec.Mysql.Ports {
		cport := corev1.ContainerPort{}
		cport.ContainerPort = svcPort.TargetPort.IntVal
		containerPorts = append(containerPorts, cport)
	}
	envlist := []corev1.EnvVar{corev1.EnvVar{Name: "MYSQL_ROOT_HOST", Value: "%"}, corev1.EnvVar{Name: "MYSQL_LOG_CONSOLE", Value: "true"}}
	envlist = append(envlist, app.Spec.Mysql.Envs...)
	container := []corev1.Container{
		corev1.Container{
			Name:            app.Name,
			Image:           app.Spec.Mysql.Image,
			Resources:       app.Spec.Mysql.Resources,
			Ports:           containerPorts,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env:             envlist,
			Command: []string{
				"/bin/bash",
				"-ecx",
				fmt.Sprintf("index=$(cat /etc/hostname | grep -o '[^-]*$');base=1000;/entrypoint.sh --server_id=$(expr $base + $index) --datadir=/var/lib/mysql --user=mysql --gtid_mode=ON --log-bin --binlog_checksum=NONE --enforce_gtid_consistency=ON --log-slave-updates=ON --binlog-format=ROW --master-info-repository=TABLE --relay-log-info-repository=TABLE --transaction-write-set-extraction=XXHASH64 --relay-log=%s-${index}-relay-bin --report-host=%s-${index}.%s --log-error-verbosity=3", app.Name, app.Name, app.Name),
			},
			// Args: []string{
			// 	"--server_id=$(expr $base + $index)",
			// },
		},
	}
	return container
}

func NewService(app *opsv1alpha1.MysqlOperator) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, schema.GroupVersionKind{
					Group:   opsv1alpha1.SchemeGroupVersion.Group,
					Version: opsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AppService",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			// Type:  corev1.ServiceTypeNodePort,
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Ports:     app.Spec.Mysql.Ports,
			Selector: map[string]string{
				"app": app.Name,
			},
		},
	}
}
