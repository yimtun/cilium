[k8s_master]
${k8s_master_eip} ansible_user=root ansible_ssh_private_key_file=../terraform/my_key ansible_become_pass=123456 ansible_ssh_common_args='-o StrictHostKeyChecking=no'

[k8s_node01]
${k8s_node01_eip}  ansible_user=root ansible_ssh_private_key_file=../terraform/my_key ansible_become_pass=123456 ansible_ssh_common_args='-o StrictHostKeyChecking=no'


[all_hosts:children]
k8s_master
k8s_node01
