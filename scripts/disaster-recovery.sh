#!/bin/bash

set -e

# Reset
RST='\033[0m'             # Text Reset

# Bold
BRed='\033[1;31m'         # Red
BGreen='\033[1;32m'       # Green
BYellow='\033[1;33m'      # Yellow
BBlue='\033[1;34m'        # Blue
BWhite='\033[1;37m'       # White

function msg_warn() {
	echo -ne "$BWhite[$BRed!$BWhite]$RST $1"
}

function msg_query() {
	echo -ne "$BWhite[$BBlue?$BWhite]$RST $1"
}

function msg_info() {
	echo -ne "$BWhite[$BYellow~$BWhite]$RST $1"
}

function msg_action() {
	echo -ne "$BWhite[$BGreen+$BWhite]$RST $1"
}

function usage() {
	echo "Usage: $0 [PROJECT] [CLUSTER] [BACKUP]"
}

function namespace_secret() {
	SOURCE_CLUSTER=$1

  # Mapping contains "clusterSource:clusterDestination" or "clusterDestination:clusterSource" to map the source and destination storage clusters
	# We need to check both syntaxes
	MAPPING=$(kubectl get --context=admin@${CLUSTER} -n ${ROOK_NAMESPACE} configmap rook-ceph-csi-mapping-config -o json | jq -r '.data."csi-mapping-config-json"')

	# Find using key
	NS=$(echo $MAPPING | jq -r ".[].ClusterIDMapping.\"$SOURCE_CLUSTER\"")

	# If not found using key, find by value
	if [[ "$NS" = null ]]; then
	NS=$(echo $MAPPING | jq -r ".[].ClusterIDMapping | to_entries[] | select(.value == \"$SOURCE_CLUSTER\")" | jq -r '.key')
	fi

	if [[ "$NS" = null ]]; then msg_warn "Failed to find the corresponding namespace\n"; exit 1; fi
	echo $NS
}

function create_remove_path_cm() {
	msg_action "Creating resource modifier ConfigMap $BWhite remove-path$RST\n"
	cat <<EOF | kubectl apply --context=admin@${CLUSTER} -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: remove-path
  namespace: ${VELERO_NAMESPACE}
data:
  remove-path.yaml: |
    version: v1
    resourceModifierRules:
    - conditions:
        groupResource: "persistentvolume"
      patches:
        - operation: remove
          path: "/spec/capacity"
        - operation: remove
          path: "/spec/claimRef"
EOF
}

# To ensure we can modify the PVCs once they're exported, we would need to change their secrets
#     - operation: replace
#      path: "/spec/csi/controllerExpandSecretRef/namespace"
#      value: "spx-aq01-test01-storage01"
#    - operation: replace
#      path: "/spec/csi/nodeStageSecretRef/namespace"
#      value: "spx-aq01-test01-storage01"

function apply_restore() {
if [[ $REPLICATION == "true" ]]; then
msg_info "Restoring from replication\n"
create_remove_path_cm
cat <<EOF | kubectl apply --context=admin@${CLUSTER} -f -
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: ${PROJECT}
  namespace: ${VELERO_NAMESPACE}
spec:
  backupName: ${BACKUP}
  uploaderConfig:
    writeSparseFiles: true
  labelSelector:
    matchLabels:
      superphenix.net/projectID: ${PROJECT}
  excludedResources:
    - persistentvolume
    - persistentvolumeclaim
  resourceModifier:
    kind: ConfigMap
    name: remove-path
EOF
fi

if [[ $REPLICATION == "false" ]]; then
msg_warn "Restoring from backup\n"
cat <<EOF | kubectl apply --context=admin@${CLUSTER} -f -
apiVersion: velero.io/v1
kind: Restore
metadata:
  name: ${PROJECT}
  namespace: ${VELERO_NAMESPACE}
spec:
  backupName: ${BACKUP}
  uploaderConfig:
    writeSparseFiles: true
  labelSelector:
    matchLabels:
      superphenix.net/projectID: ${PROJECT}
EOF
fi
	msg_action "Waiting for status completed\n"

        kubectl wait                               \
        --context=admin@${CLUSTER}                 \
        --namespace ${VELERO_NAMESPACE}            \
	--for=jsonpath='{.status.phase}'=Completed \
        --timeout=${WAIT_SECOND}s                  \
	restore.velero.io/${PROJECT}

	if [[ $? != 0 ]]; then
		msg_warn "Restore failed to complete within ${WAIT_SECOND} seconds\n"
		exit 1
	fi
}

function handle_volumes() {
	if [[ $REPLICATION == "false" ]]; then msg_info "Aborting handle_volumes as we are restoring PVCs from backups\n"; return 0; fi

	# We'll download in a temporary directory that we clean later
	TMP=$(mktemp -d); cd ${TMP}

	# Download the entire backup
	msg_action "Downloading backup $BWhite${BACKUP}$RST in ${TMP}/backup.tar.gz\n"
	velero backup download ${BACKUP}       \
	       --kubecontext=admin@${CLUSTER}  \
	       --namespace=${VELERO_NAMESPACE} \
	       --output backup.tar.gz

	# Untar the backup to explore it
	tar -xvzf backup.tar.gz >/dev/null; rm backup.tar.gz

	# We're searching for the PVs/PVCs
	PV_PATH="resources/persistentvolumes/cluster"
	PVC_PATH="resources/persistentvolumeclaims/namespaces/${PROJECT}"

	# Extract all PVs
	for FILE in $(ls ${PV_PATH}/*.json); do
		NAME="$(basename ${FILE})"
                BASE="$(basename $NAME .json)"
		NEW="$BASE.yaml"

		# Find the namespace of the PVC that owns this PV, it must be equal to the project to proceed
		OWNER_NS=$(jq -r '.spec.claimRef.namespace' $FILE)
		if [[ $OWNER_NS != $PROJECT ]]; then
			continue
		fi

		msg_info "Found PV $BWhite${BASE}$RST\n"

		# We need to remap the namespace in which the secrets are contained (for provisioning, resizing...)
	        SRC_CLUSTER=$(jq -r '.spec.csi.volumeAttributes.clusterID' $FILE)
		msg_action "Looking for corresponding namespace for source cluster $SRC_CLUSTER\n"
		export STORAGE_NAMESPACE=$(namespace_secret $SRC_CLUSTER)
		msg_info "Found namespace: $STORAGE_NAMESPACE\n"

		PATCH_PV=$(cat << EOF
[{"op": "remove", "path": "/spec/claimRef"},
{"op": "remove", "path": "/status"},
{"op": "replace", "path": "/metadata/annotations/volume.kubernetes.io~1provisioner-deletion-secret-name", "value": "$STORAGE_NAMESPACE"},
{"op": "replace", "path": "/metadata/annotations/volume.kubernetes.io~1provisioner-deletion-secret-namespace", "value": "$STORAGE_NAMESPACE"},
{"op": "replace", "path": "/spec/csi/controllerExpandSecretRef/name", "value": "$STORAGE_NAMESPACE"},
{"op": "replace", "path": "/spec/csi/controllerExpandSecretRef/namespace", "value": "$STORAGE_NAMESPACE"},
{"op": "replace", "path": "/spec/csi/nodeStageSecretRef/name", "value": "$STORAGE_NAMESPACE"},
{"op": "replace", "path": "/spec/csi/nodeStageSecretRef/namespace", "value": "$STORAGE_NAMESPACE"}]
EOF
)

		kubectl patch                             \
		        --dry-run=client                  \
		  	-f ${FILE}                        \
			-o yaml                           \
			--local                           \
			--type json                       \
			--patch "$PATCH_PV" > $NEW

		msg_action "Creating PV $BWhite$BASE$RST\n"

		set +e
		RES=$(kubectl --context=admin@${CLUSTER} create -f $NEW 2>&1)
		if [[ $? != 0 && $RES != *"AlreadyExists"* ]]; then
			msg_warn "Cannot create: $RES\n"
		fi
		set -e
	done

        sleep 3 # Let some time for controllers to process to avoid overloading

	# Extract all PVCs
	for FILE in $(ls ${PVC_PATH}/*.json); do
		NAME="$(basename ${FILE})"
		BASE="$(basename $NAME .json)"
		NEW="$BASE.yaml"
		msg_info "Found PVC $BWhite${BASE}$RST\n"

		PATCH_PVC=$(cat << EOF
[{"op": "add", "path": "/metadata/labels/velero.io~1backup-name", "value": "$BACKUP"},
{"op": "add", "path": "/metadata/labels/velero.io~1restore-name", "value": "$PROJECT"},
{"op": "remove", "path": "/metadata/ownerReferences"}]
EOF
)

		kubectl patch                             \
		        --dry-run=client                  \
		  	-f ${FILE}                        \
			-o yaml                           \
			--local                           \
			--type json                       \
			--patch "$PATCH_PVC" > $NEW

		msg_action "Creating PVC $BWhite$BASE$RST\n"

		set +e
		RES="$(kubectl --context=admin@${CLUSTER} create -f $NEW 2>&1)"
		if [[ $? != 0 && $RES != *"AlreadyExists"* ]]; then
			msg_warn "Cannot create PVC: $RES\n"
		fi
		set -e
	done

	rm -rf ${TMP}
}

function reclaim_volumes() {
	PVCS=$@
	for PVC in $PVCS; do
		msg_action "Reclaiming space for PVC $BWhite$PVC$RST\n"
		cat <<EOF | kubectl apply --context=admin@${CLUSTER} -f -
apiVersion: csiaddons.openshift.io/v1alpha1
kind: ReclaimSpaceJob
metadata:
  name: reclaim-$PVC
  namespace: $PROJECT
spec:
  target:
    persistentVolumeClaim: $PVC
EOF
	done
}

# Print usage if no parameters, or help requested
if [[ "$#" -eq 0 || ($@ == "--help") ||  $@ == "-h" ]]; then
	usage
	exit 0
fi

# SPX project we are restoring
PROJECT=$1
# Cluster on which we run the restore
CLUSTER=$2
# Backup from which we restore
BACKUP=$3
# Restore using replications
REPLICATION=$4

# Constants
VELERO_NAMESPACE="velero-system"
ROOK_NAMESPACE="rook-system"
WAIT_SECOND=10

# Verify the variables are correctly set
if [[ -z $PROJECT ]]; then usage; msg_warn "Must define an SPX project to restore\n"; exit 1; fi
if [[ -z $CLUSTER ]]; then usage; msg_warn "Must define the cluster on which to restore\n"; exit 1; fi
if [[ -z $BACKUP ]]; then usage; msg_warn "Must define the backup from which to restore\n"; exit 1; fi

# Verify if replication is enabled or not
if [[ "$REPLICATION" == "--with-replication" ]]
then
	REPLICATION="true"
	msg_warn "Replication is enabled, PVCs will not be restored from the backup, but from a VolumeReplication\n"
elif [[ ! -z "$REPLICATION" ]]; then
	msg_warn "Unexpected parameter: $REPLICATION\n"
	exit 1
fi
if [[ -z $REPLICATION ]]; then REPLICATION="false"; fi

# Prompt the user to know if they want to proceed
msg_warn "Restoring $BRed$PROJECT$RST on cluster $BRed$CLUSTER$RST from backup $BRed$BACKUP$RST\n"
if [[ $REPLICATION == "true" ]]; then msg_info "Restoring using replication\n"; fi
msg_query "Please confirm by entering \"yes\": "
read

# Verify the user agreed to proceed
if [[ $REPLY != "yes" ]]; then msg_warn "Aborting...\n"; exit 1; fi

# Start restoring the volumes
echo -e "\n\n"
msg_action "Resources restored, proceeding with PVs/PVCs...\n"
handle_volumes

# Apply Velero restore
echo -e "\n\n"
msg_action "Proceeding with restore $BWhite$PROJECT$RST\n"
apply_restore $PROJECT

if [[ $REPLICATION == "false" ]]; then
	RECLAIM_PVCS=$(kubectl get pvc -n $PROJECT --context=admin@${CLUSTER} -l velero.io/backup-name=$BACKUP -o jsonpath='{.items[*].metadata.name}')
	reclaim_volumes $RECLAIM_PVCS
fi