# cross-cloud k8s integration test

## deploy  WireGuard

```text
ansible-playbook -i hosts.ini WireGuard_install.yaml
```

## deploy k8s

```shell
ansible-playbook   -i hosts.ini  install_k8s.yaml 
```

```shell
[root@ip-10-201-1-101 ~]# kubectl  get node
NAME                                              STATUS   ROLES           AGE   VERSION
ip-10-201-1-101.ap-northeast-2.compute.internal   Ready    control-plane   13m   v1.28.2
izmj73ywmti13gblaq2jkgz                           Ready    <none>          12m   v1.28.2
vm-1-101-rockylinux                               Ready    <none>          12m   v1.28.2
[root@ip-10-201-1-101 ~]# 
```



```shell
[root@ip-10-201-1-101 ~]# kubectl get cn -o json | jq '.items[] | {
  name: .metadata.name,
  "instance-id": .spec["instance-id"],
  "multi-cloud": .spec["multi-cloud"]
}'
```

```text
{
  "name": "ip-10-201-1-101.ap-northeast-2.compute.internal",
  "instance-id": "i-057e6790a2f892725",
  "multi-cloud": {
    "cloud-provider": "aws",
    "region": "ap-northeast-2",
    "subnet-id": "subnet-0f883896cf56ac12e",
    "vpc-id": "vpc-00c1cd60f134b1c0d"
  }
}
{
  "name": "izmj73ywmti13gblaq2jkgz",
  "instance-id": "i-mj73ywmti13gblaq2jkg",
  "multi-cloud": {
    "cloud-provider": "alibabacloud",
    "region": "ap-northeast-2",
    "subnet-id": "vsw-mj7m6wj5x0mea4ll1ixid",
    "vpc-id": "vpc-mj7p8vo5kgoqff6hvac69"
  }
}
{
  "name": "vm-1-101-rockylinux",
  "instance-id": "ins-2801qizf",
  "multi-cloud": {
    "cloud-provider": "tencentcloud",
    "region": "ap-seoul",
    "subnet-id": "subnet-pv3k70xf",
    "vpc-id": "vpc-3pwstov8"
  }
}
```

## deploy test workload

```shell
cat << 'EOF' | kubectl apply -f -
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nginx
  namespace: default
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: docker.io/yimtune/nginx:1.21.6
        ports:
        - containerPort: 80
EOF
```


```shell
[root@ip-10-201-1-101 ~]# kubectl   get pod  -o wide -A
NAMESPACE     NAME                                                                      READY   STATUS    RESTARTS        AGE   IP             NODE                                              NOMINATED NODE   READINESS GATES
default       nginx-hgk4f                                                               1/1     Running   0               74s   10.201.1.80    ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
default       nginx-k2x6w                                                               1/1     Running   0               74s   10.203.1.213   izmj73ywmti13gblaq2jkgz                           <none>           <none>
default       nginx-w6s2z                                                               1/1     Running   0               74s   10.202.1.27    vm-1-101-rockylinux                               <none>           <none>
kube-system   cilium-64j99                                                              1/1     Running   0               13m   10.202.1.101   vm-1-101-rockylinux                               <none>           <none>
kube-system   cilium-726k9                                                              1/1     Running   0               13m   10.203.1.101   izmj73ywmti13gblaq2jkgz                           <none>           <none>
kube-system   cilium-gtcsz                                                              1/1     Running   2 (5m15s ago)   13m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
kube-system   cilium-operator-5c95bf6959-g9r9k                                          1/1     Running   0               13m   10.203.1.101   izmj73ywmti13gblaq2jkgz                           <none>           <none>
kube-system   coredns-5dd5756b68-4q4lb                                                  1/1     Running   0               15m   10.202.1.77    vm-1-101-rockylinux                               <none>           <none>
kube-system   coredns-5dd5756b68-9v7q4                                                  1/1     Running   0               15m   10.202.1.95    vm-1-101-rockylinux                               <none>           <none>
kube-system   etcd-ip-10-201-1-101.ap-northeast-2.compute.internal                      1/1     Running   0               15m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
kube-system   kube-apiserver-ip-10-201-1-101.ap-northeast-2.compute.internal            1/1     Running   0               15m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
kube-system   kube-controller-manager-ip-10-201-1-101.ap-northeast-2.compute.internal   1/1     Running   0               15m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
kube-system   kube-proxy-7wchv                                                          1/1     Running   0               15m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
kube-system   kube-proxy-9n5hj                                                          1/1     Running   0               14m   10.203.1.101   izmj73ywmti13gblaq2jkgz                           <none>           <none>
kube-system   kube-proxy-jjvnt                                                          1/1     Running   0               14m   10.202.1.101   vm-1-101-rockylinux                               <none>           <none>
kube-system   kube-scheduler-ip-10-201-1-101.ap-northeast-2.compute.internal            1/1     Running   0               15m   10.201.1.101   ip-10-201-1-101.ap-northeast-2.compute.internal   <none>           <none>
[root@ip-10-201-1-101 ~]# 


```




```shell
curl -SI 10.201.1.80
curl -SI 10.203.1.213
curl -SI 10.202.1.27
```


```
[root@ip-10-201-1-101 ~]# curl -SI 10.201.1.80
curl -SI 10.203.1.213
curl -SI 10.202.1.27
HTTP/1.1 200 OK
Server: nginx/1.21.6
Date: Sat, 06 Jun 2026 03:31:27 GMT
Content-Type: text/html
Content-Length: 615
Last-Modified: Tue, 25 Jan 2022 15:03:52 GMT
Connection: keep-alive
ETag: "61f01158-267"
Accept-Ranges: bytes

HTTP/1.1 200 OK
Server: nginx/1.21.6
Date: Sat, 06 Jun 2026 03:31:27 GMT
Content-Type: text/html
Content-Length: 615
Last-Modified: Tue, 25 Jan 2022 15:03:52 GMT
Connection: keep-alive
ETag: "61f01158-267"
Accept-Ranges: bytes

HTTP/1.1 200 OK
Server: nginx/1.21.6
Date: Sat, 06 Jun 2026 03:31:27 GMT
Content-Type: text/html
Content-Length: 615
Last-Modified: Tue, 25 Jan 2022 15:03:52 GMT
Connection: keep-alive
ETag: "61f01158-267"
Accept-Ranges: bytes

[root@ip-10-201-1-101 ~]# 
```




```shell
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: nginx-svc
spec:
  selector:
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  type: ClusterIP
EOF
```


```shell
[root@ip-10-201-1-101 ~]# kubectl get svc nginx-svc
NAME        TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)   AGE
nginx-svc   ClusterIP   10.96.0.197   <none>        80/TCP    8s
[root@ip-10-201-1-101 ~]# 
```



```text
curl -IS 10.96.0.197


[root@ip-10-201-1-101 ~]# curl -IS 10.96.0.197
HTTP/1.1 200 OK
Server: nginx/1.21.6
Date: Sat, 06 Jun 2026 03:32:31 GMT
Content-Type: text/html
Content-Length: 615
Last-Modified: Tue, 25 Jan 2022 15:03:52 GMT
Connection: keep-alive
ETag: "61f01158-267"
Accept-Ranges: bytes

[root@ip-10-201-1-101 ~]# 
```