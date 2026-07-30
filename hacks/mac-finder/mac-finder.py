#!/usr/bin/python3

# Shows all network interfaces of nodes specified in the file 'values.yaml'

import yaml
import redfishtoollib as rf

VALUES_PATH="values.yaml"

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

values = {}

with open(VALUES_PATH, "r") as values_file:
    values = yaml.safe_load(values_file)

# Setting options specific to redfishtool:
rft = rf.redfishtoolTransport.RfTransport()
rft.secure = "Always"
rft.subcommand = "Systems"
rft.subcommandArgv = ["", "EthernetInterfaces"]
for n in values["nodes"]:
    print(f"- Network interfaces for {n["hostname"]}")
    rft.rhost = n["ipmiIpv4"]
    rft.user = n["ipmiUser"]
    rft.password = n["ipmiPassword"]
    # Indicates that we do not select a particular interface (list all):
    rft.gotIdLevel2Optn = False
    rft.IdLevel2OptnCount = 0
    rft.matchLevel2Prop = ""
    rft.gotMatchLevel2Optn = False
    rft.IdLevel2 = ""
    rft.matchLevel2Value = ""
    interfaces = redfish_cmd(rft, f"Couldn't fetch network interfaces for {n["hostname"]}.")["Members"]
    for i in interfaces:
        if_name = i["@odata.id"].split("/")[-1]
        print(f"-- {if_name}")
        # Indicates that we select a particular interface:
        rft.gotIdLevel2Optn = True
        rft.IdLevel2OptnCount = 1
        rft.matchLevel2Prop = "Id"
        rft.gotMatchLevel2Optn = True
        rft.IdLevel2 = if_name
        rft.matchLevel2Value = rft.IdLevel2
        if_specs = redfish_cmd(rft, f"Couldn't fetch network interface {if_name} for {n["hostname"]}.")
        # Showing interface specs:
        print(f"ID: {if_specs['Id']}\nLink status: {if_specs['LinkStatus']}\nMAC address: {if_specs['MACAddress']}")