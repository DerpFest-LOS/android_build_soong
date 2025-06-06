#!/usr/bin/env python
#
# Copyright (C) 2022 The Android Open Source Project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
"""A tool for modifying privileged permission allowlists."""

import argparse
import sys
from xml.dom import minidom


class InvalidRootNodeException(Exception):
  pass


class InvalidNumberOfPrivappPermissionChildren(Exception):
  pass


def modify_allowlist(allowlist_dom, package_name):
  if allowlist_dom.documentElement.tagName != 'permissions':
    raise InvalidRootNodeException
  nodes = allowlist_dom.getElementsByTagName('privapp-permissions')
  if nodes.length != 1:
    raise InvalidNumberOfPrivappPermissionChildren
  privapp_permissions = nodes[0]
  privapp_permissions.setAttribute('package', package_name)


def parse_args():
  """Parse commandline arguments."""

  parser = argparse.ArgumentParser()
  parser.add_argument('input', help='input allowlist template file')
  parser.add_argument(
      'package_name', help='package name to use in the allowlist'
  )
  parser.add_argument('output', help='output allowlist file')

  return parser.parse_args()


def main():
  try:
    args = parse_args()
    doc = minidom.parse(args.input)
    modify_allowlist(doc, args.package_name)
    with open(args.output, 'w') as output_file:
      doc.writexml(output_file, encoding='utf-8')
  except Exception as err:
    print('error: ' + str(err), file=sys.stderr)
    sys.exit(-1)


if __name__ == '__main__':
  main()
