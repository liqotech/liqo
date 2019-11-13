from __future__ import print_function

import multiprocessing
import os
import sys
import hashlib
import json
import pprint
import shutil
import time
import zipfile

import paramiko

from collections import OrderedDict
from subprocess import TimeoutExpired
from scp import SCPClient

from config.config import Configuration
from resource_offloading.resource_offload_problem import ResourceOffloadProblem

from tests.utils.graph import Graph

# -------- Remote Test Configuration -------- #

# list of the remote hosts network addresses
remote_hosts = ["127.0.0.1", "127.0.0.1"]
# remote username for ssh
remote_username = "drone"
# location of the drone main folder on the remote hosts (both relative and absolute paths are ok)
remote_drone_path = "drone"
# local configuration file (will be copied on remote hosts)
CONF_FILE = 'config/config.ini'

# ------------------------------------------- #


p_list = list()
ssh_clients = dict()


def remote_drone_worker(_remote_host, _node_name, _log_level, _conf_file):

    ssh_clients[_node_name] = paramiko.SSHClient()

    _ssh = ssh_clients[_node_name]
    _ssh.load_system_host_keys()
    _ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    _ssh.connect(_remote_host, username=remote_username)
    # time.sleep(3)

    _stdin, _stdout, _stderr = _ssh.exec_command("cd drone" + "; "
                                                 "python3 main.py {} -l {} -d {} -o".format(_node_name,
                                                                                            _log_level,
                                                                                            _conf_file),
                                                 get_pty=True)
    _exit_status = _stdout.channel.recv_exit_status()

    lines = _stdout.readlines()
    for line in lines:
        print(line)
    lines = _stderr.readlines()
    for line in lines:
        print(line, file=sys.stderr)

    _ssh.close()


# [ Configuration ]
configuration = Configuration(CONF_FILE)
print("NODE_NUMBER:          " + str(configuration.NODES_NUMBER))
print("APP_NUMBER:           " + str(configuration.APPS_NUMBER))
print("APP_AVG_SIZE:         " + str(configuration.AVG_APP_SIZE))
print("NEIGHBOR_PROBABILITY: " + str(configuration.NEIGHBOR_PROBABILITY))
# fix hosts
if configuration.NODES_NUMBER > len(remote_hosts):
    remote_hosts = [remote_hosts[i % len(remote_hosts)] for i in range(configuration.NODES_NUMBER)]

# [ ENOP instance ]
enop = ResourceOffloadProblem()
with open(configuration.PROBLEM_INSTANCE, mode="r") as enop_file:
    enop.parse_dict(json.loads(enop_file.read()))
nodes = ["node" + str(n) for n in range(configuration.NODES_NUMBER)]
apps = ["app" + str(n) for n in range(configuration.APPS_NUMBER)]

# update enop instance according with configuration size
enop.nodes = nodes
enop.apps = apps

for app in enop.apps:
    enop.app_descriptions[app] = dict()
    base_node = enop.nodes[int(hashlib.sha256(app.encode()).hexdigest(), 16) % len(enop.nodes)]
    c_probability = configuration.AVG_APP_SIZE / len(enop.components) * 100
    app_components = [c for c in enop.components
                      if int(str(int(hashlib.sha256((app+c).encode()).hexdigest(), 16))[-2:]) < c_probability]
    if len(app_components) == 0:
        app_components.append(enop.components[0])
    enop.app_descriptions[app]['base-node'] = base_node
    enop.app_descriptions[app]['components'] = app_components
    enop.app_descriptions[app]['constraints'] = {"placement": {}}
    print("{} : {}".format(app, app_components))

enop.available_resources = dict()
known_resources_units = {"cpu": 1, "memory": 512, "bandwidth": 256, "storage": 128}
for node in enop.nodes:
    node_resources = {resource: 2**(int(hashlib.sha256(str(node).encode()).hexdigest(), 16) % 5)
                      * known_resources_units.get(resource, 1)
                      for resource in enop.resources}
    enop.available_resources[node] = node_resources

with open(configuration.PROBLEM_INSTANCE, mode="w") as enop_file:
    enop_file.write(json.dumps(enop.to_dict(), indent=4))

# prepare remote hosts
print("preparing remote hosts")
ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.load_system_host_keys()

for address in remote_hosts:
    ssh.connect(address, username=remote_username)
    # purge rabbit queues
    stdin, stdout, stderr = ssh.exec_command('cd {}'.format(remote_drone_path) + '; ' +
                                             'python3 -m scripts.purge_rabbit')
    exit_status = stdout.channel.recv_exit_status()
    print("{} {} {} {}".format(stdin, stdout.readlines(), stderr.readlines(), exit_status))

    # copy configuration, instance and topology
    scp = SCPClient(ssh.get_transport())
    scp.put([CONF_FILE, configuration.PROBLEM_INSTANCE, configuration.TOPOLOGY_FILE], remote_drone_path + "/config/")
    scp.close()

    # clean result directories
    stdin, stdout, stderr = ssh.exec_command('cd {}'.format(remote_drone_path) + '; ' +
                                             'rm -r {}'.format(configuration.RESULTS_FOLDER))
    exit_status = stdout.channel.recv_exit_status()
    print("{} {} {} {}".format(stdin, stdout.readlines(), stderr.readlines(), exit_status))
    ssh.close()

# clean result directory
shutil.rmtree(configuration.RESULTS_FOLDER, ignore_errors=True)

# load the 'nodes topology'
with open(configuration.TOPOLOGY_FILE) as topology_file:
    graph = Graph(json.load(topology_file), len(enop.nodes))
graph.print_topology()

# print total resources
total_resources = enop.get_overall_available_resources_amount()
average_resource_per_function = {r: sum([enop.get_consumption(f)[r]
                                         for f in enop.functions]) / len(enop.functions)
                                 for r in enop.resources}
average_resource_percentage_per_function = sum([average_resource_per_function[r] / total_resources[r]
                                                for r in enop.resources]) / len(enop.resources)
statistical_bundle_len = configuration.AVG_APP_SIZE
average_resource_demand = statistical_bundle_len*average_resource_percentage_per_function
print("- Resources Statistics - ")
print("Total resources: \n" + pprint.pformat(total_resources))
print("Average resources per function: \n" + pprint.pformat(average_resource_per_function))
print("Average demand percentage per function: " + str(round(average_resource_percentage_per_function, 3)))
print("Average app size: " + str(round(statistical_bundle_len, 2)))
print("Statistical average demand percentage per app: " + str(round(average_resource_demand, 3)))
print("Statistical total demand percentage: " + str(round(average_resource_demand * configuration.APPS_NUMBER, 3)))
print("- -------------------- - ")

print("- Run Orchestration - ")
for i, node_name in enumerate(enop.nodes):

    print("{} : {}".format(node_name, enop.available_resources[node_name]))

    # run sdo instance on a physical node
    host = remote_hosts[i]
    print("running instance '{}' on host '{}'".format(node_name, host))

    t = multiprocessing.Process(target=remote_drone_worker, args=(host,
                                                                  node_name,
                                                                  configuration.LOG_LEVEL,
                                                                  CONF_FILE))
    t.start()
    p_list.append(t)

killed = list()
timeout = 50
start_time = time.time()

for i, t in enumerate(p_list):
    try:
        t.join(timeout=timeout)
    except TimeoutExpired:
        t.kill()
        print("WARNING: Forcing node '{}' to terminate. Possible incomplete output.".format(enop.nodes[i]))
        killed.append('node' + str(i))
    timeout -= time.time() - start_time
    timeout = max(timeout, 1)

print(" - Collect Results - ")

result_tmp_folder = "resultTmp"
if os.path.exists(result_tmp_folder + "/" + configuration.RESULTS_FOLDER):
    shutil.rmtree(result_tmp_folder+"/"+configuration.RESULTS_FOLDER)
if not os.path.exists(result_tmp_folder+"/"+configuration.RESULTS_FOLDER):
    os.makedirs(result_tmp_folder+"/"+configuration.RESULTS_FOLDER)

# fetch result files from remote hosts
for address in remote_hosts:
    ssh.connect(address, username=remote_username)

    stdin, stdout, stderr = ssh.exec_command('cd {}/{}; '.format(remote_drone_path, configuration.RESULTS_FOLDER) +
                                             'zip validation.zip *')
    exit_status = stdout.channel.recv_exit_status()
    print("{} {} {} {}".format(stdin, stdout.readlines(), stderr.readlines(), exit_status))

    # stdin, stdout, stderr = ssh.exec_command('cd {}'.format(remote_drone_path) + '; ' + "ls *.log")
    # log_files = list(map(bytes.decode, stdout.read().splitlines()))

    scp = SCPClient(ssh.get_transport())
    # results
    scp.get('{}/{}/validation.zip'.format(remote_drone_path, configuration.RESULTS_FOLDER),
            local_path=result_tmp_folder+'{}/{}/'.format(result_tmp_folder, configuration.RESULTS_FOLDER))
    zip_ref = zipfile.ZipFile(result_tmp_folder + "/" + configuration.RESULTS_FOLDER + "/validation.zip", 'r')
    zip_ref.extractall(result_tmp_folder+"/"+configuration.RESULTS_FOLDER)
    zip_ref.close()
    os.remove(result_tmp_folder+"/"+configuration.RESULTS_FOLDER+"/validation.zip")
    # logs
    # for log_file in log_files:
    #    scp.get(remote_dragon_path + "/" + log_file)
    scp.close()
    ssh.close()

if os.path.exists(configuration.RESULTS_FOLDER):
    shutil.rmtree(configuration.RESULTS_FOLDER)

shutil.move(result_tmp_folder+"/"+configuration.RESULTS_FOLDER, os.getcwd())
shutil.rmtree(result_tmp_folder)

# fetch post process information
offloading_bundles = dict()
message_rates = dict()
private_utilities = list()
for node_name in enop.nodes:
    results_file = "{}/results_{}.json".format(configuration.RESULTS_FOLDER, node_name)

    if node_name in killed:
        private_utilities.append(0)
        offloading_bundles[node_name] = []
        message_rates[node_name] = OrderedDict([("0:0", 0)])
        continue

    try:
        with open(results_file) as f:
            results = json.loads(f.read())
            private_utilities.append(results["utility"])
            offloading_bundles[node_name] = results["offloading-bundle"]
            message_rates[node_name] = OrderedDict(results["rates"])
    except FileNotFoundError:
        continue

# sum of private utilities
print("Sum of private utilities: " + str(sum(private_utilities)))

# print assignment info
offloading_file = "{}/results.json".format(configuration.RESULTS_FOLDER)
with open(offloading_file, "w") as f:
    f.write(json.dumps(offloading_bundles, indent=4))
residual_resources = dict(enop.available_resources)
allocated_apps = set()
for node in offloading_bundles:
    for app, component, function in offloading_bundles[node]:
        allocated_apps.add(app)
        residual_resources[node] = enop.sub_resources(residual_resources[node], enop.get_consumption(function))
total_residual_resources = {r: sum([residual_resources[n][r] for n in enop.nodes]) for r in enop.resources}
total_residual_resources_percentage = sum([total_residual_resources[r] / total_resources[r]
                                           for r in enop.resources]) / len(enop.resources)
used_resources_percentage = 1 - total_residual_resources_percentage
print("Placement: \n" + pprint.pformat(offloading_bundles))
print("Residual resources: \n" + pprint.pformat(residual_resources))
print("Percentage of assigned resources: " + str(round(used_resources_percentage, 3)))
print("Percentage of successfully allocated apps: " + str(round(len(allocated_apps), 3) / configuration.APPS_NUMBER))

# calculate message rates
begin_time = min([float(next(iter(message_rates[node])).split(":")[0]) for node in message_rates])
next_begin_time = begin_time
global_rates = OrderedDict()
while len(message_rates) > 0:
    next_end_time = next_begin_time+configuration.SAMPLE_FREQUENCY
    in_range_counter = 0
    for node in message_rates:
        if len(message_rates[node]) > 0:
            in_range_keys = [k for k in message_rates[node] if float(k.split(":")[1]) <= next_end_time]
            in_range_counter += sum([message_rates[node][k] for k in in_range_keys])
            for k in in_range_keys:
                del message_rates[node][k]
    for node in dict(message_rates):
        if len(message_rates[node]) == 0:
            del message_rates[node]
    global_rates[float("{0:.3f}".format(next_end_time-begin_time))] = in_range_counter/(next_end_time-next_begin_time)
    next_begin_time = next_end_time

# print message rates
print("Message rates: \n" + pprint.pformat(global_rates))

# purge rabbit queues
for address in remote_hosts:
    ssh.connect(address, username=remote_username)
    # purge rabbitmq queues
    stdin, stdout, stderr = ssh.exec_command('cd {}'.format(remote_drone_path) + '; ' +
                                             'python3 -m scripts.purge_rabbit')
    exit_status = stdout.channel.recv_exit_status()
    print("{} {} {} {}".format(stdin, stdout.readlines(), stderr.readlines(), exit_status))
    ssh.close()
