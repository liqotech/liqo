#!/usr/bin/python

import requests
import json
import sys

if len(sys.argv) != 2:
    exit(1)

url = "https://api.github.com/repos/liqotech/liqo/pulls/" + sys.argv[1] + "/commits"

res = requests.get(url)
data = (json.loads(res.content))
print(data[0]['author']['login'])
