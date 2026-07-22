#!/usr/bin/python3

# Configures IPMI of servers for PXE boot

import os
import yaml
import json
import redfishtoollib as rf

CONFIG_PATH: str = os.getenv("CONFIG_PATH")
STATE_PATH: str = os.getenv("STATE_PATH")
CLUSTER_NAME: str = os.getenv("CLUSTER_NAME")
CLUSTER_VLAN_ID: str = os.getenv("CLUSTER_VLAN_ID")

MANUFACTURER_NAMES = {"DELL": "Dell Inc.", "LENOVO": "Lenovo"}
PXE_SETUP_SUPPORTED_MANUFACTURERS = [MANUFACTURER_NAMES["DELL"], MANUFACTURER_NAMES["LENOVO"]]

def redfish_cmd(rft, err_msg) -> dict :
    """
    Runs the Redfish command specified in 'rft' with the specified arguments.

    Parameters
    ----------
    rft: redfishtoollib.RfTransport()
        Redfish command and arguments.

    err_msg: str
        Error message to show in case of failure.

    Returns
    -------
    dict
        Response of the Redfish request.
    """
    result = rf.redfishtoolMain.runSubCmd(rft)
    if result[0] != 0:
        raise Exception(err_msg)
    if result[1].status_code == 200:
        return result[1].json()
    else:
        return {}

cluster_state_path = os.path.join(STATE_PATH, CLUSTER_NAME)
os.makedirs(cluster_state_path, exist_ok = True)

config = list()

with open(CONFIG_PATH, "r") as config_file:
    config = yaml.safe_load(config_file)
    if config == None:
        config = list()

# Removing "done" flags for removed nodes:
for f in os.listdir(cluster_state_path):
    found = False
    # Check if current flag is in the nodes list:
    for r in config:
        if f.removesuffix(".done") == r["hostname"]:
            found = True
    if not found:
        os.remove(os.path.join(cluster_state_path, f))

# Setup all nodes:
rft = rf.redfishtoolTransport.RfTransport()
rft.secure = "Always"
for r in config:
    rft.rhost = r["ipmiIpv4"]
    rft.user = r["ipmiUser"]
    rft.password = r["ipmiPassword"]

    # Skip already setup nodes:
    if not os.path.exists(os.path.join(cluster_state_path, f"{r['hostname']}.done")):
        print(f"- Setting up {r['hostname']}")
        # Get the System object and BIOS URI:
        rft.subcommand = "Systems"
        rft.subcommandArgv = list()
        # Indicates that we select the only System object:
        rft.oneOptn = True
        rft.IdOptnCount = 1
        system_object = redfish_cmd(rft, f"Could not fetch System object on {r['hostname']}.")
        bios_uri = system_object["Bios"]["@odata.id"]
        manufacturer = system_object["Manufacturer"]

        # PXE setup:
        if manufacturer in PXE_SETUP_SUPPORTED_MANUFACTURERS:
            # Get the BIOS object:
            rft.subcommand = "raw"
            rft.subcommandArgv = ["", "GET", bios_uri]
            rft.oneOptn = False
            rft.IdOptnCount = 0
            bios_object = redfish_cmd(rft, f"Could not fetch BIOS object on {r['hostname']}.")

            # Prepare patch for PXE setup:
            pxe_patch: dict = {"@Redfish.SettingsApplyTime": {"ApplyTime": "OnReset"}, "Attributes": dict()}
            if manufacturer == MANUFACTURER_NAMES["DELL"]:
                pxe_patch["Attributes"] = {"BootMode": "Uefi", "PxeDev1EnDis": "Enabled", "PxeDev1Protocol": "IPv4", "PxeDev1Interface": r["pxeInterfaceName"]}
            elif manufacturer == MANUFACTURER_NAMES["LENOVO"]:
                pxe_patch["Attributes"] = {"BootModes_SystemBootMode": "UEFIMode", "NetworkStackSettings_NetworkStack": "Enable", "NetworkStackSettings_IPv4PXESupport": "Enable"}

            if r["useVlan"]:
                if manufacturer == MANUFACTURER_NAMES["DELL"]:
                    pxe_patch["Attributes"]["PxeDev1VlanEnDis"] = "Enabled"
                    pxe_patch["Attributes"]["PxeDev1VlanId"] = int(CLUSTER_VLAN_ID)
                else:
                    print(f"WARNING: Setting up PXE VLAN automatically is not supported on '{manufacturer}' machines yet, it might need to be set up manually.")

            # Apply patch:
            rft.subcommand = "raw"
            rft.subcommandArgv = ["", "PATCH", bios_object["@Redfish.Settings"]["SettingsObject"]["@odata.id"]]
            rft.oneOptn = False
            rft.IdOptnCount = 0
            rft.headers = {"content-type": "application/json"}
            rft.requestData = str(json.dumps(pxe_patch))
            rft.blocking = False
            redfish_cmd(rft, f"Could not set up PXE on {r['hostname']}.")
        else:
            print(f"WARNING: Setting up PXE automatically is not supported on '{manufacturer}' machines yet, it might need to be set up manually. Only next boot option will be set.")

        # Set next boot option to PXE:
        rft.subcommand = "Systems"
        rft.subcommandArgv = ["", "setBootOverride", "Once", "Pxe"]
        rft.oneOptn = True
        rft.IdOptnCount = 1
        rft.headers = {}
        rft.requestData = ""
        rft.blocking = True
        redfish_cmd(rft, f"Could not set next boot option to PXE on {r['hostname']}.")

        # Reboot if already on, else power on:
        power_state = system_object["PowerState"]
        reset_type = "ForceRestart"
        if power_state == "Off":
            reset_type = "On"

        rft.subcommandArgv = ["", "reset", reset_type]
        redfish_cmd(rft, f"Could not reboot/power on {r['hostname']}.")

        # Mark node done:
        f = open(os.path.join(cluster_state_path, f"{r['hostname']}.done"), "w") ; f.close()

        print(f"- {r['hostname']} set up successfully")
    else:
        print(f"- Skipping {r['hostname']} as it is already set up")