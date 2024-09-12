#!/usr/bin/python3

# This python script is to fix ansible error in CI test. It's not a bug of ansible, but a side-effect of a different change
# Will remove it later if we do not see ansible error in CI

import os
import sys

for handle in (sys.stdin, sys.stdout, sys.stderr):
    try:
        fd = handle.fileno()
    except Exception as e:
        print(f"Error: {e}")
        continue

    os.set_blocking(fd, True)

print("Blocking I/O is set for all file descriptors")
