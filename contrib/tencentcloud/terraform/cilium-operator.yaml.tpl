apiVersion: apps/v1
kind: Deployment
metadata:
  name: cilium-operator
  namespace: kube-system
  labels:
    app: cilium-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cilium-operator
  template:
    metadata:
      labels:
        app: cilium-operator
    spec:
      hostNetwork: true
      tolerations:
        - operator: Exists
      priorityClassName: system-cluster-critical
      containers:
        - name: cilium-operator
          image: docker.io/yimtune/operator-tencentcloud:tencentcloud-dev
          imagePullPolicy: Always
          command: ["cilium-operator-tencentcloud"]
          args:
            - --k8s-kubeconfig-path=/etc/kubernetes/admin.conf
            - --debug
          env:
            - name: TENCENTCLOUD_SECRET_ID
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-credentials
                  key: secret-id
            - name: TENCENTCLOUD_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: tencentcloud-credentials
                  key: secret-key
            - name: TENCENTCLOUD_REGION
              value: "${region}"
            - name: TENCENTCLOUD_VPC_ID
              value: "${vpc_id}"
            - name: TENCENTCLOUD_SUBNET
              value: "${subnet_id}"
          volumeMounts:
            - name: admin-kubeconfig
              mountPath: /etc/kubernetes/admin.conf
              readOnly: true
      volumes:
        - name: admin-kubeconfig
          hostPath:
            path: /etc/kubernetes/admin.conf
            type: File