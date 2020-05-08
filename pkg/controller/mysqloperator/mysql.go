package mysqloperator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	opsv1alpha1 "github.com/jwping/mysql-operator/pkg/apis/ops/v1alpha1"
	"github.com/oracle/mysql-operator/pkg/cluster/innodb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retry int = 60
	// namespace string = "default"
	// name      string = "mysql-operator"
)

var Logger = log.WithValues("MySQL Cluster Controller")

func (m *mysqlBelong) createClusters(r *ReconcileMysqlOperator) ([]string, bool) {
	var badPod []string

	podList := m.waitForCluster(r)
	for _, pod := range podList.Items {
		if m.checkCluster2(r, pod.Name) {
			if cidr, err := whitelistCIDR(pod.Status.PodIP); err != nil {
				log.Error(err, pod.Name)
				badPod = append(badPod, pod.Name)
				continue
			} else {
				var cpython string
				if m.Instance.Spec.Mysql.MultiMaster {
					cpython = fmt.Sprintf("print(dba.create_cluster('%s', {'force':True, 'ipWhitelist':'%s', 'memberSslMode':'REQUIRED', 'multiMaster':True}).status())", strings.Replace(m.Instance.Namespace, "-", "_", -1), cidr)
				} else {
					cpython = fmt.Sprintf("print(dba.create_cluster('%s', {'ipWhitelist':'%s', 'memberSslMode':'REQUIRED'}).status())", strings.Replace(m.Instance.Namespace, "-", "_", -1), cidr)
				}

				passwd := getPassword(m.Instance.Spec.Mysql.Envs)
				uri := fmt.Sprintf("root:%s@%s.%s.%s:3306", passwd, pod.Name, m.Instance.Name, pod.Namespace)
				stdout, err := r.run.run(context.TODO(), "mysqlsh", []string{"--no-wizard", "--uri", uri, "--py", "-e", cpython})
				if err != nil {
					log.Error(err, pod.Name)
					continue
				}
				Logger.Info(stdout.String())
				return badPod, true

			}

		}
		badPod = append(badPod, pod.Name)
		continue
	}
	return badPod, false
}

func (m *mysqlBelong) InstanceClusters(r *ReconcileMysqlOperator) (badpod []string) {
	passwd := getPassword(m.Instance.Spec.Mysql.Envs)
	masterdns := m.findMaster(r)
	if masterdns == "" {
		return
	}

	uri := fmt.Sprintf("root:%s@%s", passwd, masterdns)

	status := m.getStatus(r, uri)
	mastername := strings.Split(masterdns, ".")[0]

	podList := m.waitForCluster(r)
	for _, pod := range podList.Items {
		if pod.Name == mastername {
			continue
		}
		poddns := fmt.Sprintf("%s.%s:3306", pod.Name, m.Instance.Name)
		if _, ok := status.DefaultReplicaSet.Topology[poddns]; !ok {
			if m.checkCluster2(r, pod.Name) {
				cidr, err := whitelistCIDR(pod.Status.PodIP)
				if err != nil {
					log.Error(err, pod.Name)
					badpod = append(badpod, pod.Name)
					continue
				}

				ipython := fmt.Sprintf("print(dba.get_cluster('%s').add_instance('root:%s@%s', {'ipWhitelist':'%s', 'recoveryMethod': 'incremental'}))", strings.Replace(m.Instance.Namespace, "-", "_", -1), passwd, poddns, cidr)

				stdout, err := r.run.run(context.TODO(), "mysqlsh", []string{"--no-wizard", "--uri", uri, "--py", "-e", ipython})
				if err != nil {
					log.Error(err, pod.Name)
					badpod = append(badpod, pod.Name)
				}
				Logger.Info(stdout.String())
			} else {
				badpod = append(badpod, pod.Name)
			}
		}
	}

	return
}

func (m *mysqlBelong) checkCluster2(r *ReconcileMysqlOperator, podname string) bool {
	passwd := getPassword(m.Instance.Spec.Mysql.Envs)
	podDns := fmt.Sprintf("%s.%s.%s", podname, m.Instance.Name, m.Instance.Namespace)
	for i := 0; i < retry; i++ {
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
		defer cancel()
		_, err := r.run.run(ctx, "mysqladmin", []string{"-h", podDns,
			"--protocol", "tcp",
			"-u", "root",
			fmt.Sprintf("-p%s", passwd),
			"status"})
		if err == nil {
			return true
		}
		time.Sleep(time.Second * 10)
	}
	return false
}

func (m *mysqlBelong) getStatus(r *ReconcileMysqlOperator, uri string) (status *innodb.ClusterStatus) {
	status = &innodb.ClusterStatus{}
	gpython := fmt.Sprintf("print(dba.get_cluster('%s').status())", strings.Replace(m.Instance.Namespace, "-", "_", -1))
	stdout, err := r.run.run(context.TODO(), "mysqlsh", []string{"--no-wizard", "--uri", uri, "--py", "-e", gpython})
	if err != nil {
		return
	}
	json.Unmarshal([]byte(sanitizeJSON(stdout.String())), status)

	return
}

func (m *mysqlBelong) findMaster(r *ReconcileMysqlOperator) string {
	passwd := getPassword(m.Instance.Spec.Mysql.Envs)
	podlist := m.waitForCluster(r)
	for _, pod := range podlist.Items {
		uri := fmt.Sprintf("root:%s@%s.%s.%s", passwd, pod.Name, m.Instance.Name, m.Instance.Namespace)
		status := m.getStatus(r, uri)
		if reflect.DeepEqual(*status, innodb.ClusterStatus{}) {
			continue
		}
		return status.GroupInformationSourceMember
	}
	return ""
}

func (m *mysqlBelong) removePod(r *ReconcileMysqlOperator, repodlist []string) {
	for _, podname := range repodlist {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: m.Instance.Namespace,
				Name:      podname,
			},
		}
		m.RemoveExample(r, podname)
		r.client.Delete(context.Background(), pod)
		// Logger.Info("Pod", podname, "Restart operation", r.client.Delete(context.Background(), pod))
	}
}

func (m *mysqlBelong) clearAll(r *ReconcileMysqlOperator) {
	podlist := m.waitForCluster(r)
	for _, pod := range podlist.Items {
		m.removePod(r, []string{pod.Name})
	}
}

func (m *mysqlBelong) waitForCluster(r *ReconcileMysqlOperator) *corev1.PodList {
	for {
		if podList, err := m.getPodList(r); err == nil && m.Instance.Spec.Mysql.Size == int32(len(podList.Items)) {
			// reqLogger.Info("All nodes started")
			return podList
		}
		time.Sleep(time.Second)
	}
}

func (m *mysqlBelong) getPodList(r *ReconcileMysqlOperator) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(m.Instance.Namespace),
		client.MatchingLabels(map[string]string{"mysql": "operator"}),
	}

	if err := r.client.List(context.TODO(), podList, listOpts...); err != nil {
		return nil, err
	}
	return podList, nil

}

type mysqlBelong struct {
	Passwd   string
	Instance *opsv1alpha1.MysqlOperator
	// Namespace string
	// Name      string
}

func new(r *ReconcileMysqlOperator) *mysqlBelong {
	for {
		time.Sleep(time.Second * 3)
		instance, err := getInstance(r)
		if err != nil {
			continue
		}
		return &mysqlBelong{Passwd: getPassword(instance.Spec.Mysql.Envs), Instance: instance}
	}
}

func (r *ReconcileMysqlOperator) MysqlManager() {

	defer Logger.Error(fmt.Errorf("MysqlManager"), " Abnormal exit!")

	for {

		mysqlbelong := new(r)
		if mastername := mysqlbelong.findMaster(r); mastername == "" {
			Logger.Info("Cluster not found, judge cluster not created")
			if badpodlist, boval := mysqlbelong.createClusters(r); boval == false {
				Logger.Info("Cluster creation failed, clear all")
				// mysqlbelong.clearAll(r)
			} else {
				mysqlbelong.removePod(r, badpodlist)
				badpodlist = mysqlbelong.InstanceClusters(r)
				Logger.Info("Cluster created successfully, clean up abnormal nodes")
				mysqlbelong.removePod(r, badpodlist)
			}
		} else {
			passwd := getPassword(mysqlbelong.Instance.Spec.Mysql.Envs)
			uri := fmt.Sprintf("root:%s@%s", passwd, mastername)
			status := mysqlbelong.getStatus(r, uri)
			podlist := mysqlbelong.waitForCluster(r)
			if len(podlist.Items) != len(status.DefaultReplicaSet.Topology) {
				badpodlist := mysqlbelong.InstanceClusters(r)
				Logger.Info("Try to add a new node and clean up the abnormal status node")
				mysqlbelong.removePod(r, badpodlist)
			}
			for podname, podstatus := range status.DefaultReplicaSet.Topology {
				if podstatus.Status != "ONLINE" && podstatus.Status != "RECOVERING" {
					Logger.Info(podname, "Abnormal state, cleaning")
					mysqlbelong.removePod(r, []string{strings.TrimSuffix(podname, ":3306")})
				}
			}
		}

	}
}

func (m *mysqlBelong) RemoveExample(r *ReconcileMysqlOperator, podname string) {
	passwd := getPassword(m.Instance.Spec.Mysql.Envs)
	mastername := m.findMaster(r)
	if mastername == "" {
		return
	}
	uri := fmt.Sprintf("root:%s@%s", passwd, mastername)

	stdout, err := r.run.run(context.TODO(), "mysqlsh", []string{"--no-wizard", "--uri", uri, "--py", "-e", fmt.Sprintf("print(dba.get_cluster('%s').remove_instance('%s', {'force': True}))", strings.Replace(m.Instance.Namespace, "-", "_", -1), podname)})
	if err != nil {
		return
	}
	Logger.Info(stdout.String())

}
