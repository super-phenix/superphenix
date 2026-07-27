package vm

import v1 "kubevirt.io/api/core/v1"

var (
	CPU_VALUE_LIST          = []int{1, 2, 4, 8, 16, 32}
	MEMORY_VALUE_LIST       = []int{1, 2, 4, 8, 16, 32, 64}
	RUN_STRATEGY_VALUE_LIST = []string{
		string(v1.RunStrategyAlways),
		string(v1.RunStrategyRerunOnFailure),
		string(v1.RunStrategyOnce),
		string(v1.RunStrategyManual),
		string(v1.RunStrategyHalted),
	}
)

const DEFAULT_CLOUD_INIT = `#cloud-config
ssh_pwauth: true
users:
  - name: spx
    groups: sudo
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    shell: /bin/bash
chpasswd:
  expire: true
  users:
    - {name: spx, password: "<generated_password>", type: text}
write_files:
  - path: /etc/systemd/resolved.conf.d/dns_servers.conf
    content: |
      [Resolve]
      DNS=1.1.1.1 1.0.0.1
    permissions: '0644'
runcmd:
  - systemctl restart systemd-resolved
`
