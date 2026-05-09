apiVersion: v1
kind: Secret
metadata:
  name: tencentcloud-credentials
  namespace: kube-system
type: Opaque
stringData:
  secret-id: "${secret_id}"
  secret-key: "${secret_key}"