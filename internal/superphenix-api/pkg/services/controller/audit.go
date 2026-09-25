package controller

// ParamEffectiveID is the URL param carrying the product ID on audited routes.
const ParamEffectiveID = "effectiveId"

// Audit actions specific to the products, next to router.ActionCreate, ActionUpdate and ActionDelete.
const (
	ActionStart                = "start"
	ActionStop                 = "stop"
	ActionStopForce            = "stop-force"
	ActionRestart              = "restart"
	ActionMountContainerDisk   = "container-disk.mount"
	ActionUnmountContainerDisk = "container-disk.unmount"
	ActionUnmount              = "unmount"
	ActionRestore              = "restore"
	ActionClone                = "clone"
	ActionReinstallEssentials  = "reinstall-essentials"
	ActionUpgrade              = "upgrade"
)
