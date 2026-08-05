# KaaS resources

This document details the top-level resources (CRDs) deployed by the SFS-KAAS helm chart, explains their role 
and lists the child-ressources that get created from them.

## Cluster

- [Cluster](https://cluster-api.sigs.k8s.io/user/concepts#cluster)
- KubevirtCluster

## Controlplane

- KamajiControlPlane
  - [TenantControlPlane](https://kamaji.clastix.io/concepts/tenant-control-plane/)
    - Secret
    - Deployment
    - Configmap
    - Service
    - Ingress *(not used, will be disabled ASAP)*

## Controlplane datastore

Persistent storage for the controlpane data. Only deployed when a dedicated datastore is demanded (a mutualized datastore is used by default).

- [DataStore](https://kamaji.clastix.io/concepts/datastore/) *(not namespaced)*
- EtcdCluster
  - Service
  - ConfigMap
  - StatefulSet
- Issuer
- Certificate

## Controlplane Gateway

Forwards incoming trafic from worker nodes or cluster administrators to the corresponding controlplane.

- Gateway
- TLSRoute
- Certificate

## Tenant worker VMs

- KubeadmConfigTemplate
- KubevirtMachineTemplate
- [MachineDeployment](https://cluster-api.sigs.k8s.io/user/concepts#machinedeployment)
  - [MachineSet](https://cluster-api.sigs.k8s.io/user/concepts#machineset)
    - [Machine](https://cluster-api.sigs.k8s.io/user/concepts#machine)
      - KubevirtMachine *(based on KubevirtMachineTemplate)*
      - KubeadmConfig *(based on KubeadmConfigTemplate)*
        - Secret

## Network

Ensures that tenant clusters remain operational by allowing the required connections.  
Can be disabled to allow for stricter user-defined policies.

- [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/)

## Kubevirt CSI

Provides Storage and snapshot functionalities to the tenant cluster by interfacing with the  
workload cluster's storage solution.

- ServiceAccount
- Role
- RoleBinding
- ClusterRole *(not namespaced)*
- ClusterRoleBinding *(not namespaced)*
- ConfigMap
- Deployment

## Tenant setup job

Deploys essential components in the tenant cluster using the kaas-essentials helm chart.

- ConfigMap
- Job
