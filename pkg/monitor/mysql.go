package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	opsv1alpha1 "github.com/jwping/mysql-operator/pkg/apis/ops/v1alpha1"
	"github.com/oracle/mysql-operator/pkg/cluster/innodb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("mysql_monitor")

type MysqlMonitor struct {
	client   client.Client
	Instance *opsv1alpha1.MysqlOperator
	ctx      context.Context

	flag bool
}

func New(client client.Client, ctx context.Context) *MysqlMonitor {
	return &MysqlMonitor{client: client, ctx: ctx}
}

func (m *MysqlMonitor) CreateClusters() {
	mainPodDns := fmt.Sprintf("%s-0.%s.%s", m.Instance.Name, m.Instance.Name, m.Instance.Namespace)
	passwd := getPassword(m.Instance.Spec.Mysql.Envs)

	for {
		if isDatabaseRunning(mainPodDns, passwd) {
			log.Info("Database ready!")
			// time.Sleep(time.Second)
			break
		}
		fmt.Printf("\n\n等待数据库启动\n\n")
		time.Sleep(time.Second)
	}
	log.Info("Creating InnoDB cluster")

	podList := waitForCluster(m.client, m.Instance)
	fmt.Printf("podList: %v\n", podList)

	// run := NewRunner(utilexec.New(), fmt.Sprintf("root:%s@%s:3306", passwd, mainPodDns))
	run := newRunner(utilexec.New(), fmt.Sprintf("root:%s@%s:33306", passwd, "192.168.14.132"))

	// var cpython string
	if cidr, err := whitelistCIDR(podList.Items[0].Status.PodIP); err != nil {
		// log.Error(err, "Unable to create cluster")
		fmt.Printf("\nUnable to create cluster: %v\n", err)
		return
	} else {
		var cpython string
		if m.Instance.Spec.Mysql.MultiMaster {
			cpython = fmt.Sprintf("print(dba.create_cluster('%s', {'force':True, 'ipWhitelist':'%s', 'memberSslMode':'REQUIRED', 'multiMaster':True}).status())", m.Instance.Namespace, cidr)
		} else {
			cpython = fmt.Sprintf("print(dba.create_cluster('%s', {'ipWhitelist':'%s', 'memberSslMode':'REQUIRED'}).status())", m.Instance.Namespace, cidr)
		}

		fmt.Printf("\n\npython: %s\n\n", cpython)

		stdout, err := run.run(cpython)
		if err != nil {
			// log.Error(err, "Create command execution exception")
			fmt.Printf("\nCreate command execution exception: %v\n", err)
		}
		fmt.Printf("\n\nstdout: %s\n", stdout.String())
		// log.Info(stdout.String())
	}

	for _, pod := range podList.Items {
		if cidr, err := whitelistCIDR(pod.Status.PodIP); err != nil {
			log.Error(err, "Node", pod.Status.PodIP, "does not belong to cluster network")
			continue
		} else {
			// ipython := fmt.Sprintf("print(dba.get_cluster('%s').add_instance('root:%s@%s.%s.%s', {'ipWhitelist':'%s', 'memberSslMode':'REQUIRED', 'recoveryMethod': 'incremental'}))", m.Instance.Namespace, passwd, pod.Name, m.Instance.Name, m.Instance.Name, cidr)
			ipython := fmt.Sprintf("print(dba.get_cluster('%s').add_instance('root:%s@%s.%s.%s', {'ipWhitelist':'%s', 'memberSslMode':'REQUIRED', 'recoveryMethod': 'incremental'}))", m.Instance.Namespace, passwd, pod.Name, m.Instance.Name, m.Instance.Name, cidr)
			fmt.Printf("\n\nipython: %s\n\n", ipython)
			stdout, err := run.run(ipython)
			if err != nil {
				log.Error(err, "Exception in adding cluster node")
			}
			log.Info(stdout.String())
		}
	}

	log.Info("InnoDB initialization completed")
}

func (m *MysqlMonitor) StartMysqlManager() {
	if m.flag {
		return
	}
	m.flag = true
	go MysqlManager(m)
}

func MysqlManager(m *MysqlMonitor) {

	defer func() { m.flag = false }()
	log.Info("启动mysqlmanager！")

	for {
		// if m.waitfor() {
		// 	return
		// }
		// if reflect.DeepEqual(*m.Instance, opsv1alpha1.MysqlOperator{}) {
		// 	fmt.Printf("\n\nm.Instance为空！\n\n")
		// 	// if *m.Instance.Status == (cachev1alpha1.MemcachedStatus{}) {
		// 	continue
		// }

		passwd := getPassword(m.Instance.Spec.Mysql.Envs)

		var run *runner
		podlist, _ := getPodList(m.client, m.Instance)
		for _, pod := range podlist.Items {
			fmt.Println(pod)
			// run = NewRunner(utilexec.New(), fmt.Sprintf("root:%s@%s.%s.%s", passwd, pod.Name, m.Instance.Name, m.Instance.Namespace))
			run = newRunner(utilexec.New(), fmt.Sprintf("root:%s@%s:33306", passwd, "192.168.14.132"))

			for {
				python := fmt.Sprintf("print(dba.get_cluster('%s').status())", m.Instance.Namespace)
				fmt.Printf("\n\npython: %s\n\n", python)

				stdout, err := run.run(python)
				if err != nil {
					log.Error(err, "Failed to get cluster status")
					break
					// goto waitfor
				}
				// reqLoggem.Info(stdout.String())
				status := &innodb.ClusterStatus{}
				err = json.Unmarshal([]byte(sanitizeJSON(stdout.String())), status)
				if err != nil {
					log.Error(err, "decoding cluster status output")
					break
					// goto waitfor
				}
				fmt.Printf("获取到的status为：%v\n", status.DefaultReplicaSet)
				for key, value := range status.DefaultReplicaSet.Topology {
					if value.Status != "ONLINE" && value.Status != "RECOVERING" {
						pod := &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: m.Instance.Namespace,
								Name:      strings.TrimSuffix(key, ":3306"),
							},
						}
						log.Info("Pod ", key, "Restart operation: ", m.client.Delete(context.Background(), pod))
					}
					// fmt.Printf("key: %s - value: %v\n", strings.TrimSuffix(key, ":3306"), value)
				}
				// waitfor:
				if m.waitfor() {
					return
				}
				// time.Sleep(time.Minute)

				// return status, nil
			}
		}
	}
}

func (m *MysqlMonitor) waitfor() bool {
	select {
	case <-m.ctx.Done():
		return true
	case <-time.Tick(time.Minute):
		return false
	}
}
