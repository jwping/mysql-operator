package mysqloperator

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
	"k8s.io/apimachinery/pkg/types"
	utilexec "k8s.io/utils/exec"
)

func getInstance(r *ReconcileMysqlOperator) (*opsv1alpha1.MysqlOperator, error) {
	instance := &opsv1alpha1.MysqlOperator{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: "mysql-operator", Name: "mysql-operator"}, instance)
	return instance, err
}

func getPassword(envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == "MYSQL_ROOT_PASSWORD" {
			return env.Value
		}
	}
	return ""
}

func isDatabaseRunning(hostDns, passwd string) bool {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancel()
	err := utilexec.New().CommandContext(ctx,
		"mysqladmin",
		"-h", hostDns,
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
}

func sanitizeJSON(json string) string {
	return strings.Replace(json, "\\'", "'", -1)
}

func (r *runner) run(ctx context.Context, command string, args []string) (*bytes.Buffer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := r.exec.CommandContext(ctx, command, args...)

	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)

	err := cmd.Run()
	if err != nil {
		return stdout, fmt.Errorf("err: %v - stderr: %s\n", err, stderr.String())
	}
	return stdout, nil
}
