package monitor

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	opsv1alpha1 "github.com/jwping/mysql-operator/pkg/apis/ops/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	utilexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getPassword(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == "MYSQL_ROOT_PASSWORD" {
			return env.Value
		}
	}
	return ""
}

func waitForCluster(c client.Client, instance *opsv1alpha1.MysqlOperator) *corev1.PodList {
	for {
		if podList, err := getPodList(c, instance); err == nil && instance.Spec.Mysql.Size == int32(len(podList.Items)) {
			// reqLogger.Info("All nodes started")
			return podList
		}
		time.Sleep(time.Second)
	}
}

func getPodList(c client.Client, instance *opsv1alpha1.MysqlOperator) (*corev1.PodList, error) {
	// Update the Memcached status with the pod names
	// List the pods for this memcached's deployment
	// podip := &corev1.PodIP{}
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		// client.InNamespace(instance.Namespace),
		client.InNamespace(instance.Namespace),
		// client.MatchingLabels(labelsForMemcached(instance.Name)),
		client.MatchingLabels(map[string]string{"mysql": "operator"}),
	}
	fmt.Printf("listOpts: %v\n", listOpts)

	// if err = r.client.List(context.TODO(), podip, listOpts...); err != nil {
	if err := c.List(context.TODO(), podList, listOpts...); err != nil {
		// reqLogger.Error(err, "Failed to list pods", "Memcached.Namespace", namespace, "Memcached.Name", instance.Name)
		return nil, err
	}
	return podList, nil

}

func isDatabaseRunning(hostDns, passwd string) bool {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancel()
	// chanb := make(chan bool)
	fmt.Printf("\n\nhostDns: %s\npasswd: %s\n\n", hostDns, passwd)
	err := utilexec.New().CommandContext(ctx,
		"mysqladmin",
		// "-h", hostDns,
		"-h", "192.168.14.132",
		"-P", "33306",
		"--protocol", "tcp",
		"-u", "root",
		fmt.Sprintf("-p%s", passwd),
		// "-p", passwd,
		// os.ExpandEnv("-p$MYSQL_ROOT_PASSWORD"),
		"status",
	).Run()
	return err == nil
}

// WhitelistCIDR returns the CIDR range to whitelist for GR based on the Pod's IP.
func whitelistCIDR(ip string) (string, error) {
	var privateRanges []*net.IPNet

	for _, addrRange := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10", // IPv4 shared address space (RFC 6598), improperly used by kops
	} {
		_, block, _ := net.ParseCIDR(addrRange)
		privateRanges = append(privateRanges, block)
	}

	for _, block := range privateRanges {
		if block.Contains(net.ParseIP(ip)) {
			return block.String(), nil
		}
	}

	return "", fmt.Errorf("pod IP %q is not a private IPv4 address", ip)
}

// runner implements Interface in terms of exec("mysqlsh").
type runner struct {
	mu   sync.Mutex
	exec utilexec.Interface
	// command string

	// uri is Uniform Resource Identifier of the MySQL instance to connect to.
	// Format: [user[:pass]]@host[:port][/db].
	uri string
}

// New creates a new MySQL Shell Interface.
func newRunner(exec utilexec.Interface, uri string) *runner {
	return &runner{exec: exec, uri: uri}
}

func sanitizeJSON(json string) string {
	return strings.Replace(json, "\\'", "'", -1)
}

func (r *runner) run(python string) (*bytes.Buffer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	args := []string{"--no-wizard", "--uri", r.uri, "--py", "-e", python}

	cmd := r.exec.CommandContext(context.TODO(), "mysqlsh", args...)

	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)

	// glog.V(6).Infof("Running command: mysqlsh %v", args)
	err := cmd.Run()
	if err != nil {
		return stdout, fmt.Errorf("err: %v - stderr: %s\n", err, stderr.String())
	}
	return stdout, nil
}
