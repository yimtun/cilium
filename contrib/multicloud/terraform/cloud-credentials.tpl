apiVersion: v1
kind: Secret
metadata:
  name: cloud-credentials
  namespace: kube-system
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: ${aws_access_key_id}
  AWS_SECRET_ACCESS_KEY: ${aws_secret_access_key}
  ALIBABA_CLOUD_ACCESS_KEY_ID: ${alibaba_cloud_access_key_id}
  ALIBABA_CLOUD_ACCESS_KEY_SECRET: ${alibaba_cloud_access_key_secret}
  TENCENTCLOUD_SECRET_ID: ${tencentcloud_secret_id}
  TENCENTCLOUD_SECRET_KEY: ${tencentcloud_secret_key}
  GOOGLE_CREDENTIALS_JSON: |
    ${indent(4, trimspace(google_credentials_json))}
  AZURE_CREDENTIALS_JSON: |
    ${indent(4, trimspace(azure_credentials_json))}
  AZURE_RESOURCE_GROUP: multicloud-test
