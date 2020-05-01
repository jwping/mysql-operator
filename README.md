# MySQL Operator

MySQL [Operator-SDK](https://github.com/operator-framework/operator-sdk)创建，配置和管理在Kubernetes上运行的MySQL InnoDB集群。它不适用于MySQL NDB群集。

**目前发布仅为测试版，且尚未支持PVC，您应对数据和数据库集群的运行负责。在第一个主要版本发布之前，可能会有向后不兼容的更改。**

## 特征

当前版本仅提供以下核心功能（2020-05-01）：

* 轻松创建和删除Kubernetes中的高可用MySQL InnoDB集群

## 要求

* Kubernetes 1.8.0 +

## 入门

### 1、向目标kubernetes集群注册CRD

```
[root@master mysql-operator]# kubectl create -f deploy/crds/ops.iflytek.com_mysqloperators_crd.yaml 
customresourcedefinition.apiextensions.k8s.io/mysqloperators.ops.iflytek.com created
```

请注意：完成此操作后，有两种方法可以运行该operator，我这里只介绍在集群中部署：

- 作为Kubernetes集群中的部署
- 集群外的Go程序

### 2、设置RBAC并部署Mysql-operator

```shell
[root@master mysql-operator]# kubectl apply -f deploy
namespace/mysql-operator created
deployment.apps/mysql-operator created
role.rbac.authorization.k8s.io/mysql-operator created
rolebinding.rbac.authorization.k8s.io/mysql-operator created
serviceaccount/mysql-operator created

#您应看到下列四个资源
[root@master mysql-operator]# kubectl get -n mysql-operator all
NAME                                  READY   STATUS    RESTARTS   AGE
pod/mysql-operator-859dcfd55b-spjrl   1/1     Running   0          7m55s

NAME                             TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
service/mysql-operator-metrics   ClusterIP   10.100.74.194   <none>        8383/TCP,8686/TCP   6m27s

NAME                             READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/mysql-operator   1/1     1            1           7m55s

NAME                                        DESIRED   CURRENT   READY   AGE
replicaset.apps/mysql-operator-859dcfd55b   1         1         1       7m55s
```

我这里是创建了一个名为mysql-operator的名称空间，并将RBAC等全部丢了进去，接下来启动的mysql实例也将在这个名称空间内。

### 3、创建Mysql实例

```shell
root@jwping:/mnt/d/GoPath/src/github.com/jwping/mysql-operator# cat example/innodb-cluster.yaml
apiVersion: ops.iflytek.com/v1alpha1
kind: MysqlOperator
metadata:
  name: mysql-operator
  namespace: mysql-operator
spec:
  mysql:
    size: 3
    image: mysql/mysql-server:8.0.19
    multiMaster: true
    envs:
    - name: MYSQL_ROOT_PASSWORD
      value: R00Tmysql
    ports:
      - port: 3306
        targetPort: 3306


[root@master mysql-operator]# kubectl apply -f example/innodb-cluster.yaml 
mysqloperator.ops.iflytek.com/mysql-operator created
```

* 例子中的`size`指定为3，通过修改该参数可指定mysql实例的数量
* 请确保使用的的镜像为`mysql-server`而非`mysql`
* 通过`multiMaster: true`可指定开启多主模式
* 修改`MYSQL_ROOT_PASSWORD`的`value`可指定Mysql Root用户的密码，默认为`R00Tmysql`

**请勿修改namespace，除非您在进行二次开发，如遇任何问题可提交Issues**

### 4、查看operator日志

```shell
[root@master mysql-operator]# kubectl logs -f -n mysql-operator mysql-operator-859dcfd55b-9lvp2

...
{"level":"info","ts":1588232647.5787194,"logger":"controller_mysqloperator","msg":"Reconciling MysqlOperator","Request.Namespace":"mysql-operator","Request.Name":"mysql-operator"}
{"level":"info","ts":1588232647.7192173,"logger":"controller_mysqloperator","msg":"Reconciling MysqlOperator","Request.Namespace":"mysql-operator","Request.Name":"mysql-operator"}
{"level":"info","ts":1588232652.1367311,"logger":"controller_mysqloperator","msg":"Cluster not found, judge cluster not created"}
{"level":"info","ts":1588232667.1739464,"logger":"controller_mysqloperator","msg":"\nA new InnoDB cluster will be created on instance 'mysql-operator-0.mysql-operator.mysql-operator:3306'.\n\n\n\n\n{\"clusterName\": \"mysql_operator\", \"defaultReplicaSet\": {\"name\": \"default\", \"ssl\": \"REQUIRED\", \"status\": \"OK_NO_TOLERANCE\", \"statusText\": \"Cluster is NOT tolerant to any failures.\", \"topology\": {\"mysql-operator-0.mysql-operator:3306\": {\"address\": \"mysql-operator-0.mysql-operator:3306\", \"mode\": \"R/W\", \"readReplicas\": {}, \"replicationLag\": null, \"role\": \"HA\", \"status\": \"ONLINE\", \"version\": \"8.0.19\"}}, \"topologyMode\": \"Multi-Primary\"}, \"groupInformationSourceMember\": \"mysql-operator-0.mysql-operator:3306\"}\n"}
{"level":"info","ts":1588232675.4434505,"logger":"controller_mysqloperator","msg":"\n\n\n\n\n\nNone\n"}
{"level":"info","ts":1588232691.781496,"logger":"controller_mysqloperator","msg":"\n\n\n\n\n\nNone\n"}
{"level":"info","ts":1588232691.7815282,"logger":"controller_mysqloperator","msg":"Cluster created successfully, clean up abnormal nodes"}
```

如果一切顺利，您应看到如上日志。



## 二次开发

### 修改启动参数

默认的启动参数如下（位于GOPATH/src/github.com/jwping/mysql-operator/pkg/controller/operator.go:72）：
```shell
--server_id=<pod.Name.Id>
--datadir=/var/lib/mysql
--user=mysql
--gtid_mode=ON
--log-bin
--binlog_checksum=NONE
--enforce_gtid_consistency=ON
--log-slave-updates=ON
--binlog-format=ROW
--master-info-repository=TABLE
--relay-log-info-repository=TABLE
--transaction-write-set-extraction=XXHASH64
--relay-log=<pod.Name>-relay-bin
--report-host=<pod.Name>.<app.Name>
--log-error-verbosity=3
```

该文件下定义了operator的具体实现

### 修改innodb.go

这里我引用并修改了oracle仓库下的mysql-operator部分代码，所以您在二次开发时需要修改对应的结构体或定制（GOPATH/src/github.com/oracle/mysql-operator/pkg/cluster/innodb/innodb.go:116）：

```shell
#ClusterStatus结构体增加GroupInformationSourceMember成员

// ClusterStatus represents the status of an InnoDB cluster
type ClusterStatus struct {
	ClusterName                  string     `json:"clusterName"`
	DefaultReplicaSet            ReplicaSet `json:"defaultReplicaSet"`
	GroupInformationSourceMember string     `json:"groupInformationSourceMember"`
}
```

**如果您使用了go mod模块，那么请在GOPATH/pkg/mod/github.com/oracle目录下进行修改！**



### 编译镜像

构建mysql-operator映像并将其推送到镜像仓库。以下示例使用阿里云作为源。

```shell
[root@master mysql-operator]# operator-sdk build registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1
INFO[0002] Building OCI image registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1 
Sending build context to Docker daemon  86.57MB
Step 1/7 : FROM registry.access.redhat.com/ubi8/ubi:latest
 ---> 8121a9f5303b
Step 2/7 : ENV OPERATOR=/usr/local/bin/mysql-operator     USER_UID=1001     USER_NAME=mysql-operator
 ---> Using cache
 ---> 0bd7a34dbbb4
...
Step 7/7 : USER ${USER_UID}
 ---> Running in 9360f99b7437
Removing intermediate container 9360f99b7437
 ---> b826281dd49c
Successfully built b826281dd49c
Successfully tagged registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1
INFO[0661] Operator build complete. 

[root@master mysql-operator]# docker push registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1
```

接下来替换operator.yaml文件中的字符串`REPLACE_IMAGE`为您的镜像`registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1`，请确认您的`operator.yaml`文件已成功更新。

```shell
[root@master mysql-operator]# sed -i 's|REPLACE_IMAGE|registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1|g' deploy/operator.yaml

[root@master mysql-operator]# cat deploy/operator.yaml
...
spec:
      serviceAccountName: mysql-operator
      containers:
        - name: mysql-operator
          # Replace this with the built image name
          image: registry.cn-shanghai.aliyuncs.com/anshan/mysql-operator:v0.0.1
          command:
          - mysql-operator
          imagePullPolicy: Always
...
```

接下来您可以按照入门步骤来进行部署Mysql-operator

**重点提示：**确保您的群集能够拉取推送到仓库的映像。