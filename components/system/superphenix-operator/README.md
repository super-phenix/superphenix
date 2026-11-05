# superphenix-operator

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

## Introduction

The Superphenix Operator is the core component responsible for managing the lifecycle of Superphenix clusters.

- **Management Cluster Deployment**: This operator is installed on the management cluster of SPX.
- **Cluster Lifecycle Management**: It handles the installation and lifecycle management of Superphenix clusters using the `Cluster` Custom Resource Definition (CRD).
- **Deployment Engine**: It deploys an ArgoCD instance as the deployment engine used to bootstrap all Superphenix clusters.
- **Self-Management**: The management configuration (API, Console, etc.) is handled via the Helm chart, which deploys a `Cluster` resource pointing to the local Kubernetes cluster where the operator is running.
- **Versatility**: The operator can be used to install various types of clusters, including hyperconverged or decoupled Availability Zones (AZs), storage clusters, and workload clusters.

## Values

<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>affinity</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Affinity rules for pod assignment.</td>
		</tr>
		<tr>
			<td>clusters</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Superphenix clusters to be managed by this operator. You can either create the Cluster CRs by hand or use this field to define your clusters from the chart.</td>
		</tr>
		<tr>
			<td>config</td>
			<td>object</td>
			<td><pre lang="json">
{
  "argocd": {
    "chart": {
      "url": "",
      "version": ""
    },
    "ha": {
      "enabled": false
    },
    "values": {}
  },
  "clustersConfigMap": {
    "name": "superphenix-clusters-config"
  },
  "database": {
    "autoConnect": true,
    "host": "",
    "name": "",
    "password": "",
    "port": "",
    "user": ""
  },
  "disableVersionValidation": false,
  "enableHTTP2": false,
  "gitops": {
    "path": ".",
    "repoURL": "",
    "targetRevision": "HEAD"
  },
  "syncPeriod": "5m",
  "syncTimeout": "15m",
  "system": {
    "chartName": "superphenix-system",
    "repoURL": "ghcr.io/super-phenix/charts",
    "version": ""
  },
  "talosManager": {
    "chart": {
      "url": "ghcr.io/super-phenix/charts",
      "version": "0.1.0"
    }
  },
  "telemetry": {
    "disabled": false
  },
  "valuesConfigMap": {
    "name": "superphenix-mgmt-values"
  }
}
</pre>
</td>
			<td>Superphenix operator configuration</td>
		</tr>
		<tr>
			<td>config.argocd.chart</td>
			<td>object</td>
			<td><pre lang="json">
{
  "url": "",
  "version": ""
}
</pre>
</td>
			<td>Override the ArgoCD chart location and version.</td>
		</tr>
		<tr>
			<td>config.argocd.ha</td>
			<td>object</td>
			<td><pre lang="json">
{
  "enabled": false
}
</pre>
</td>
			<td>ArgoCD configuration.</td>
		</tr>
		<tr>
			<td>config.argocd.ha.enabled</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Enable HA mode for ArgoCD deployment.</td>
		</tr>
		<tr>
			<td>config.argocd.values</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Arbitrary Helm values merged under the "argocd" ConfigMap key.</td>
		</tr>
		<tr>
			<td>config.clustersConfigMap</td>
			<td>object</td>
			<td><pre lang="json">
{
  "name": "superphenix-clusters-config"
}
</pre>
</td>
			<td>ConfigMap where all clusters will append their configuration.</td>
		</tr>
		<tr>
			<td>config.database.autoConnect</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Automatically connect to the default deployed PostgreSQL by probing the 'postgres' secret.</td>
		</tr>
		<tr>
			<td>config.database.host</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The host of the Superphenix Database.</td>
		</tr>
		<tr>
			<td>config.database.name</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The name of the Superphenix Database.</td>
		</tr>
		<tr>
			<td>config.database.password</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The password of the Superphenix Database.</td>
		</tr>
		<tr>
			<td>config.database.port</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The port of the Superphenix Database.</td>
		</tr>
		<tr>
			<td>config.database.user</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The user of the Superphenix Database.</td>
		</tr>
		<tr>
			<td>config.disableVersionValidation</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Disable validation of versions entirely.</td>
		</tr>
		<tr>
			<td>config.enableHTTP2</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Enable HTTP/2 for the metrics and webhook servers.</td>
		</tr>
		<tr>
			<td>config.gitops.path</td>
			<td>string</td>
			<td><pre lang="json">
"."
</pre>
</td>
			<td>Default path for project GitOps applications.</td>
		</tr>
		<tr>
			<td>config.gitops.repoURL</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Default repository URL for project GitOps applications.</td>
		</tr>
		<tr>
			<td>config.gitops.targetRevision</td>
			<td>string</td>
			<td><pre lang="json">
"HEAD"
</pre>
</td>
			<td>Default target revision for project GitOps applications.</td>
		</tr>
		<tr>
			<td>config.syncPeriod</td>
			<td>string</td>
			<td><pre lang="json">
"5m"
</pre>
</td>
			<td>Interval at which to periodically resync sub-applications.</td>
		</tr>
		<tr>
			<td>config.syncTimeout</td>
			<td>string</td>
			<td><pre lang="json">
"15m"
</pre>
</td>
			<td>Duration after which an in-progress sub-application sync is considered stuck.</td>
		</tr>
		<tr>
			<td>config.system</td>
			<td>object</td>
			<td><pre lang="json">
{
  "chartName": "superphenix-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "version": ""
}
</pre>
</td>
			<td>Default system chart deployed by the cluster controller on every managed cluster. This can be overridden per cluster in the Cluster CR.</td>
		</tr>
		<tr>
			<td>config.system.chartName</td>
			<td>string</td>
			<td><pre lang="json">
"superphenix-system"
</pre>
</td>
			<td>Name of the system chart.</td>
		</tr>
		<tr>
			<td>config.system.repoURL</td>
			<td>string</td>
			<td><pre lang="json">
"ghcr.io/super-phenix/charts"
</pre>
</td>
			<td>Repository URL for the system chart.</td>
		</tr>
		<tr>
			<td>config.system.version</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Version of the system chart. If none provided, inherits from the operator chart.</td>
		</tr>
		<tr>
			<td>config.talosManager</td>
			<td>object</td>
			<td><pre lang="json">
{
  "chart": {
    "url": "ghcr.io/super-phenix/charts",
    "version": "0.1.0"
  }
}
</pre>
</td>
			<td>talos-manager chart deployed by the cluster controller for every managed cluster.</td>
		</tr>
		<tr>
			<td>config.talosManager.chart.url</td>
			<td>string</td>
			<td><pre lang="json">
"ghcr.io/super-phenix/charts"
</pre>
</td>
			<td>Repository URL for the talos-manager chart.</td>
		</tr>
		<tr>
			<td>config.talosManager.chart.version</td>
			<td>string</td>
			<td><pre lang="json">
"0.1.0"
</pre>
</td>
			<td>Version of the talos-manager chart.</td>
		</tr>
		<tr>
			<td>config.telemetry</td>
			<td>object</td>
			<td><pre lang="json">
{
  "disabled": false
}
</pre>
</td>
			<td>Telemetry settings.</td>
		</tr>
		<tr>
			<td>config.telemetry.disabled</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Disable sending anonymous telemetry.</td>
		</tr>
		<tr>
			<td>config.valuesConfigMap</td>
			<td>object</td>
			<td><pre lang="json">
{
  "name": "superphenix-mgmt-values"
}
</pre>
</td>
			<td>General ConfigMap holding Helm values overrides for all management components.</td>
		</tr>
		<tr>
			<td>fullnameOverride</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>String to fully override fullname template.</td>
		</tr>
		<tr>
			<td>health</td>
			<td>object</td>
			<td><pre lang="json">
{
  "probeBindAddress": ":8081"
}
</pre>
</td>
			<td>Health check configuration.</td>
		</tr>
		<tr>
			<td>health.probeBindAddress</td>
			<td>string</td>
			<td><pre lang="json">
":8081"
</pre>
</td>
			<td>The address the health probe endpoint binds to.</td>
		</tr>
		<tr>
			<td>image.pullPolicy</td>
			<td>string</td>
			<td><pre lang="json">
"IfNotPresent"
</pre>
</td>
			<td>Pull policy for the operator image.</td>
		</tr>
		<tr>
			<td>image.repository</td>
			<td>string</td>
			<td><pre lang="json">
"ghcr.io/super-phenix/superphenix-operator"
</pre>
</td>
			<td>Repository for the operator image.</td>
		</tr>
		<tr>
			<td>image.tag</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Overrides the image tag whose default is the chart appVersion.</td>
		</tr>
		<tr>
			<td>imagePullSecrets</td>
			<td>list</td>
			<td><pre lang="json">
[]
</pre>
</td>
			<td>Secrets for pulling the operator image.</td>
		</tr>
		<tr>
			<td>installOnClusterWithoutCNI</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Install the operator on a Kubernetes cluster with no CNI configured.  This setting is required when bootstrapping a Superphenix cluster where the operator is responsible for installing the CNI (e.g. via the superphenix-system chart). Without a CNI, Kubernetes nodes are marked as 'NotReady' and pods cannot get an IP address from the pod network. Enabling this option: 1. Adds tolerations to the operator deployment so it can be scheduled on 'NotReady' nodes. 2. Configures the operator pod to use the host network (`hostNetwork: true`). 3. Configures the managed ArgoCD instance to use the host network (`hostNetwork: true`). 4. Sets the deployment strategy to 'Recreate' to avoid port conflicts during updates.  IMPORTANT: This should be set to false once the Superphenix cluster has been fully installed and a CNI is available and functional.</td>
		</tr>
		<tr>
			<td>leaderElection.enabled</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Enable leader election for the operator, necessary if running multiple replicas.</td>
		</tr>
		<tr>
			<td>management</td>
			<td>object</td>
			<td><pre lang="json">
{
  "availabilityZone": "",
  "lifecycle": {
    "cleanupOnDeletion": false,
    "manual": false,
    "pause": false
  },
  "region": "",
  "systemConfiguration": {},
  "systemLocation": {},
  "version": ""
}
</pre>
</td>
			<td>Superphenix management cluster configuration. This will create a Cluster resource for the management cluster, pointing to the Kubernetes cluster where the operator is installed. Only management configuration can be defined here. If you also want to install a Storage/Workload/Hyperconverged cluster on the Kubernetes cluster where the operator is installed, you need to create a separate Cluster CR manually or use the 'clusters' field below.</td>
		</tr>
		<tr>
			<td>management.availabilityZone</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Availability zone where the management cluster is located. This is optional, as the cluster will not be visible to end users. Defaults to "management".</td>
		</tr>
		<tr>
			<td>management.lifecycle</td>
			<td>object</td>
			<td><pre lang="json">
{
  "cleanupOnDeletion": false,
  "manual": false,
  "pause": false
}
</pre>
</td>
			<td>Lifecycle settings for the management cluster.</td>
		</tr>
		<tr>
			<td>management.lifecycle.cleanupOnDeletion</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether to clean up the cluster when it gets deleted.</td>
		</tr>
		<tr>
			<td>management.lifecycle.manual</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether to disable the autosync of all applications on the management cluster.</td>
		</tr>
		<tr>
			<td>management.lifecycle.pause</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether to pause the synchronization of the Superphenix stack on the management cluster.</td>
		</tr>
		<tr>
			<td>management.region</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Region where the management cluster is located. This is optional, as the cluster will not be visible to end users. Defaults to "management".</td>
		</tr>
		<tr>
			<td>management.systemConfiguration</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>System configuration for the management stack. This configuration is passed to the superphenix-system chart.</td>
		</tr>
		<tr>
			<td>management.systemLocation</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Override the default Superphenix system chart location. This is useful if you want to use a custom chart.</td>
		</tr>
		<tr>
			<td>management.version</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Override the default Superphenix system chart version. This is useful if you want to use a custom chart.</td>
		</tr>
		<tr>
			<td>metrics</td>
			<td>object</td>
			<td><pre lang="json">
{
  "bindAddress": "0",
  "secure": true
}
</pre>
</td>
			<td>Metrics configuration.</td>
		</tr>
		<tr>
			<td>metrics.bindAddress</td>
			<td>string</td>
			<td><pre lang="json">
"0"
</pre>
</td>
			<td>The address the metric endpoint binds to. Use "0" to disable.</td>
		</tr>
		<tr>
			<td>metrics.secure</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Whether to secure the metrics endpoint with TLS.</td>
		</tr>
		<tr>
			<td>nameOverride</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>String to partially override fullname template.</td>
		</tr>
		<tr>
			<td>networkPolicy</td>
			<td>object</td>
			<td><pre lang="json">
{
  "enabled": true
}
</pre>
</td>
			<td>Network policy configuration.</td>
		</tr>
		<tr>
			<td>nodeSelector</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Node selector for pod assignment.</td>
		</tr>
		<tr>
			<td>podAnnotations</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Annotations for the operator pods.</td>
		</tr>
		<tr>
			<td>podSecurityContext</td>
			<td>object</td>
			<td><pre lang="json">
{
  "runAsNonRoot": true,
  "seccompProfile": {
    "type": "RuntimeDefault"
  }
}
</pre>
</td>
			<td>Pod-level security context.</td>
		</tr>
		<tr>
			<td>podSecurityContext.runAsNonRoot</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Whether to run as a non-root user.</td>
		</tr>
		<tr>
			<td>podSecurityContext.seccompProfile.type</td>
			<td>string</td>
			<td><pre lang="json">
"RuntimeDefault"
</pre>
</td>
			<td>Type of seccomp profile to use.</td>
		</tr>
		<tr>
			<td>replicaCount</td>
			<td>int</td>
			<td><pre lang="json">
1
</pre>
</td>
			<td>Number of replicas for the operator deployment.</td>
		</tr>
		<tr>
			<td>resources</td>
			<td>object</td>
			<td><pre lang="json">
{
  "limits": {
    "cpu": "500m"
  },
  "requests": {
    "cpu": "10m",
    "memory": "64Mi"
  }
}
</pre>
</td>
			<td>Resource limits and requests for the operator container.</td>
		</tr>
		<tr>
			<td>resources.limits.cpu</td>
			<td>string</td>
			<td><pre lang="json">
"500m"
</pre>
</td>
			<td>CPU limit for the operator.</td>
		</tr>
		<tr>
			<td>resources.requests.cpu</td>
			<td>string</td>
			<td><pre lang="json">
"10m"
</pre>
</td>
			<td>CPU request for the operator.</td>
		</tr>
		<tr>
			<td>resources.requests.memory</td>
			<td>string</td>
			<td><pre lang="json">
"64Mi"
</pre>
</td>
			<td>Memory request for the operator.</td>
		</tr>
		<tr>
			<td>securityContext</td>
			<td>object</td>
			<td><pre lang="json">
{
  "allowPrivilegeEscalation": false,
  "capabilities": {
    "drop": [
      "ALL"
    ]
  },
  "readOnlyRootFilesystem": true
}
</pre>
</td>
			<td>Container-level security context.</td>
		</tr>
		<tr>
			<td>securityContext.allowPrivilegeEscalation</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Whether to allow privilege escalation.</td>
		</tr>
		<tr>
			<td>securityContext.readOnlyRootFilesystem</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Whether to mount the container's root filesystem as read-only.</td>
		</tr>
		<tr>
			<td>serviceAccount.annotations</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Annotations to add to the service account.</td>
		</tr>
		<tr>
			<td>serviceAccount.create</td>
			<td>bool</td>
			<td><pre lang="json">
true
</pre>
</td>
			<td>Specifies whether a service account should be created.</td>
		</tr>
		<tr>
			<td>serviceAccount.name</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>The name of the service account to use. If not set and create is true, a name is generated using the fullname template.</td>
		</tr>
		<tr>
			<td>tolerations</td>
			<td>list</td>
			<td><pre lang="json">
[]
</pre>
</td>
			<td>Tolerations for pod assignment.</td>
		</tr>
		<tr>
			<td>toolbox</td>
			<td>object</td>
			<td><pre lang="json">
{
  "affinity": {},
  "enabled": false,
  "image": {
    "pullPolicy": "IfNotPresent",
    "repository": "ghcr.io/super-phenix/superphenix-toolbox",
    "tag": ""
  },
  "imagePullSecrets": [],
  "nodeSelector": {},
  "podAnnotations": {},
  "podSecurityContext": {},
  "resources": {
    "limits": {
      "cpu": "200m",
      "memory": "256Mi"
    },
    "requests": {
      "cpu": "10m",
      "memory": "64Mi"
    }
  },
  "securityContext": {},
  "tolerations": []
}
</pre>
</td>
			<td>Superphenix toolbox configuration. The toolbox is an optional standalone Deployment deployed alongside the operator. It provides standard tooling (bash, jq, kubectl, helm, k9s and more) and mounts the kubeconfig of every cluster managed by the operator, allowing administrators to access all clusters from a single container.</td>
		</tr>
		<tr>
			<td>toolbox.affinity</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Affinity rules for toolbox pod assignment.</td>
		</tr>
		<tr>
			<td>toolbox.enabled</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Enable the toolbox deployment. Disabled by default.</td>
		</tr>
		<tr>
			<td>toolbox.image.pullPolicy</td>
			<td>string</td>
			<td><pre lang="json">
"IfNotPresent"
</pre>
</td>
			<td>Pull policy for the toolbox image.</td>
		</tr>
		<tr>
			<td>toolbox.image.repository</td>
			<td>string</td>
			<td><pre lang="json">
"ghcr.io/super-phenix/superphenix-toolbox"
</pre>
</td>
			<td>Repository for the toolbox image.</td>
		</tr>
		<tr>
			<td>toolbox.image.tag</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Overrides the image tag whose default is the chart appVersion.</td>
		</tr>
		<tr>
			<td>toolbox.imagePullSecrets</td>
			<td>list</td>
			<td><pre lang="json">
[]
</pre>
</td>
			<td>Secrets for pulling the toolbox image.</td>
		</tr>
		<tr>
			<td>toolbox.nodeSelector</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Node selector for toolbox pod assignment.</td>
		</tr>
		<tr>
			<td>toolbox.podAnnotations</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Annotations for the toolbox pods.</td>
		</tr>
		<tr>
			<td>toolbox.podSecurityContext</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Pod-level security context for the toolbox.</td>
		</tr>
		<tr>
			<td>toolbox.resources</td>
			<td>object</td>
			<td><pre lang="json">
{
  "limits": {
    "cpu": "200m",
    "memory": "256Mi"
  },
  "requests": {
    "cpu": "10m",
    "memory": "64Mi"
  }
}
</pre>
</td>
			<td>Resource limits and requests for the toolbox container.</td>
		</tr>
		<tr>
			<td>toolbox.securityContext</td>
			<td>object</td>
			<td><pre lang="json">
{}
</pre>
</td>
			<td>Container-level security context for the toolbox.</td>
		</tr>
		<tr>
			<td>toolbox.tolerations</td>
			<td>list</td>
			<td><pre lang="json">
[]
</pre>
</td>
			<td>Tolerations for toolbox pod assignment.</td>
		</tr>
	</tbody>
</table>

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)