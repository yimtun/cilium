[k8s_tc]
${k8s_tc_eip} ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[k8s_ali]
${k8s_ali_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[k8s_aws_bootstrap]
k8s_aws_node ansible_host=${k8s_aws_eip} ansible_user=rocky ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[k8s_aws]
${k8s_aws_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_tc]
${wg_tc_eip} ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[wg_ali]
${wg_ali_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[wg_aws_bootstrap]
wg_aws_node ansible_host=${wg_aws_eip} ansible_user=rocky ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_aws]
${wg_aws_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[k8s_gcp_bootstrap]
k8s_gcp_node ansible_host=${k8s_gcp_eip} ansible_user=rocky ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[k8s_gcp]
${k8s_gcp_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_gcp_bootstrap]
wg_gcp_node ansible_host=${wg_gcp_eip} ansible_user=rocky ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_gcp]
${wg_gcp_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[k8s_azure_bootstrap]
k8s_azure_node ansible_host=${k8s_azure_eip} ansible_user=almalinux ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[k8s_azure]
${k8s_azure_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_azure_bootstrap]
wg_azure_node ansible_host=${wg_azure_eip} ansible_user=almalinux ansible_ssh_private_key_file=../terraform/my_key ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible

[wg_azure]
${wg_azure_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key  ansible_ssh_common_args='-o StrictHostKeyChecking=no' ansible_remote_tmp=/tmp/.ansible


[all_hosts:children]
k8s_tc
k8s_ali
k8s_aws
k8s_gcp
k8s_azure

[wg_nodes:children]
wg_tc
wg_ali
wg_aws
wg_gcp
wg_azure

[aws_bootstrap:children]
k8s_aws_bootstrap
wg_aws_bootstrap

[gcp_bootstrap:children]
k8s_gcp_bootstrap
wg_gcp_bootstrap

[azure_bootstrap:children]
k8s_azure_bootstrap
wg_azure_bootstrap
