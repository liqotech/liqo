import hashlib
import json
import pprint
import shutil
import subprocess

import time
from collections import OrderedDict
from subprocess import TimeoutExpired

from config.config import Configuration
from resource_offloading.resource_offload_problem import ResourceOffloadProblem

from scripts import purge_rabbit
from tests.utils.graph import Graph

p_list = list()

# [ Configuration ]
CONF_FILE = 'config/config.ini'
configuration = Configuration(CONF_FILE)
print("NODE_NUMBER:          " + str(configuration.NODES_NUMBER))
print("APP_NUMBER:           " + str(configuration.APPS_NUMBER))
print("APP_AVG_SIZE:         " + str(configuration.AVG_APP_SIZE))

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

# purge rabbit queues
purge_rabbit.purge_queues(["{}-drone".format(node) for node in nodes],
                          configuration.BROKER_ADDRESS, configuration.USERNAME, configuration.PASSWORD)

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
for node_name in enop.nodes:
    print("{} : {}".format(node_name, enop.available_resources[node_name]))
    p = subprocess.Popen(["python3", "main.py", node_name, "-l", configuration.LOG_LEVEL, "-d", CONF_FILE, "-o"])
    p_list.append(p)

killed = list()
timeout = 50
step_time = time.time()

for i, p in enumerate(p_list):
    try:
        p.wait(timeout=timeout)
    except TimeoutExpired:
        print(timeout)
        p.kill()
        print("WARNING: Forcing node '{}' to terminate. Possible incomplete output.".format(enop.nodes[i]))
        killed.append(enop.nodes[i])
    new_step_time = time.time()
    timeout -= new_step_time - step_time
    step_time = new_step_time
    timeout = max(timeout, 1)

print(" - Collect Results - ")
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
purge_rabbit.purge_queues(["{}-drone".format(node) for node in nodes],
                          configuration.BROKER_ADDRESS, configuration.USERNAME, configuration.PASSWORD)
