#!/usr/bin/env python
#
# Copyright (C) 2018 The Android Open Source Project
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
"""A tool for inserting values from the build system into a manifest."""

import argparse
import sys
from xml.dom import minidom


from manifest import *

def parse_args():
  """Parse commandline arguments."""

  parser = argparse.ArgumentParser()
  parser.add_argument('--minSdkVersion', default='', dest='min_sdk_version',
                      help='specify minSdkVersion used by the build system')
  parser.add_argument('--replaceMaxSdkVersionPlaceholder', default='', dest='max_sdk_version',
                      help='specify maxSdkVersion used by the build system')
  parser.add_argument('--targetSdkVersion', default='', dest='target_sdk_version',
                      help='specify targetSdkVersion used by the build system')
  parser.add_argument('--raise-min-sdk-version', dest='raise_min_sdk_version', action='store_true',
                      help='raise the minimum sdk version in the manifest if necessary')
  parser.add_argument('--library', dest='library', action='store_true',
                      help='manifest is for a static library')
  parser.add_argument('--uses-library', dest='uses_libraries', action='append',
                      help='specify additional <uses-library> tag to add. android:required is set to true')
  parser.add_argument('--optional-uses-library', dest='optional_uses_libraries', action='append',
                      help='specify additional <uses-library> tag to add. android:required is set to false')
  parser.add_argument('--uses-non-sdk-api', dest='uses_non_sdk_api', action='store_true',
                      help='manifest is for a package built against the platform')
  parser.add_argument('--logging-parent', dest='logging_parent', default='',
                      help=('specify logging parent as an additional <meta-data> tag. '
                            'This value is ignored if the logging_parent meta-data tag is present.'))
  parser.add_argument('--use-embedded-dex', dest='use_embedded_dex', action='store_true',
                      help=('specify if the app wants to use embedded dex and avoid extracted,'
                            'locally compiled code. Must not conflict if already declared '
                            'in the manifest.'))
  parser.add_argument('--extract-native-libs', dest='extract_native_libs',
                      default=None, type=lambda x: (str(x).lower() == 'true'),
                      help=('specify if the app wants to use embedded native libraries. Must not conflict '
                            'if already declared in the manifest.'))
  parser.add_argument('--has-no-code', dest='has_no_code', action='store_true',
                      help=('adds hasCode="false" attribute to application. Ignored if application elem '
                            'already has a hasCode attribute.'))
  parser.add_argument('--test-only', dest='test_only', action='store_true',
                      help=('adds testOnly="true" attribute to application. Assign true value if application elem '
                            'already has a testOnly attribute.'))
  parser.add_argument('--override-placeholder-version', dest='new_version',
                      help='Overrides the versionCode if it\'s set to the placeholder value of 0')
  parser.add_argument('input', help='input AndroidManifest.xml file')
  parser.add_argument('output', help='output AndroidManifest.xml file')
  return parser.parse_args()


def raise_min_sdk_version(doc, min_sdk_version, target_sdk_version, library):
  """Ensure the manifest contains a <uses-sdk> tag with a minSdkVersion.

  Args:
    doc: The XML document.  May be modified by this function.
    min_sdk_version: The requested minSdkVersion attribute.
    target_sdk_version: The requested targetSdkVersion attribute.
    library: True if the manifest is for a library.
  Raises:
    RuntimeError: invalid manifest
  """

  manifest = parse_manifest(doc)

  for uses_sdk in get_or_create_uses_sdks(doc, manifest):
    # Get or insert the minSdkVersion attribute.  If it is already present, make
    # sure it as least the requested value.
    min_attr = uses_sdk.getAttributeNodeNS(android_ns, 'minSdkVersion')
    if min_attr is None:
      min_attr = doc.createAttributeNS(android_ns, 'android:minSdkVersion')
      min_attr.value = min_sdk_version
      uses_sdk.setAttributeNode(min_attr)
    else:
      if compare_version_gt(min_sdk_version, min_attr.value):
        min_attr.value = min_sdk_version

    # Insert the targetSdkVersion attribute if it is missing.  If it is already
    # present leave it as is.
    target_attr = uses_sdk.getAttributeNodeNS(android_ns, 'targetSdkVersion')
    if target_attr is None:
      target_attr = doc.createAttributeNS(android_ns, 'android:targetSdkVersion')
      if library:
        # TODO(b/117122200): libraries shouldn't set targetSdkVersion at all, but
        # ManifestMerger treats minSdkVersion="Q" as targetSdkVersion="Q" if it
        # is empty.  Set it to something low so that it will be overridden by the
        # main manifest, but high enough that it doesn't cause implicit
        # permissions grants.
        target_attr.value = '16'
      else:
        target_attr.value = target_sdk_version
      uses_sdk.setAttributeNode(target_attr)


def add_logging_parent(doc, logging_parent_value):
  """Add logging parent as an additional <meta-data> tag.

  Args:
    doc: The XML document. May be modified by this function.
    logging_parent_value: A string representing the logging
      parent value.
  Raises:
    RuntimeError: Invalid manifest
  """
  manifest = parse_manifest(doc)

  logging_parent_key = 'android.content.pm.LOGGING_PARENT'
  for application in get_or_create_applications(doc, manifest):
    indent = get_indent(application.firstChild, 2)

    last = application.lastChild
    if last is not None and last.nodeType != minidom.Node.TEXT_NODE:
      last = None

    if not find_child_with_attribute(application, 'meta-data', android_ns,
                                     'name', logging_parent_key):
      ul = doc.createElement('meta-data')
      ul.setAttributeNS(android_ns, 'android:name', logging_parent_key)
      ul.setAttributeNS(android_ns, 'android:value', logging_parent_value)
      application.insertBefore(doc.createTextNode(indent), last)
      application.insertBefore(ul, last)
      last = application.lastChild

    # align the closing tag with the opening tag if it's not
    # indented
    if last and last.nodeType != minidom.Node.TEXT_NODE:
      indent = get_indent(application.previousSibling, 1)
      application.appendChild(doc.createTextNode(indent))


def add_uses_libraries(doc, new_uses_libraries, required):
  """Add additional <uses-library> tags

  Args:
    doc: The XML document. May be modified by this function.
    new_uses_libraries: The names of libraries to be added by this function.
    required: The value of android:required attribute. Can be true or false.
  Raises:
    RuntimeError: Invalid manifest
  """

  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    indent = get_indent(application.firstChild, 2)

    last = application.lastChild
    if last is not None and last.nodeType != minidom.Node.TEXT_NODE:
      last = None

    for name in new_uses_libraries:
      if find_child_with_attribute(application, 'uses-library', android_ns,
                                   'name', name) is not None:
        # If the uses-library tag of the same 'name' attribute value exists,
        # respect it.
        continue

      ul = doc.createElement('uses-library')
      ul.setAttributeNS(android_ns, 'android:name', name)
      ul.setAttributeNS(android_ns, 'android:required', str(required).lower())

      application.insertBefore(doc.createTextNode(indent), last)
      application.insertBefore(ul, last)

    # align the closing tag with the opening tag if it's not
    # indented
    if application.lastChild.nodeType != minidom.Node.TEXT_NODE:
      indent = get_indent(application.previousSibling, 1)
      application.appendChild(doc.createTextNode(indent))


def add_uses_non_sdk_api(doc):
  """Add android:usesNonSdkApi=true attribute to <application>.

  Args:
    doc: The XML document. May be modified by this function.
  Raises:
    RuntimeError: Invalid manifest
  """

  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    attr = application.getAttributeNodeNS(android_ns, 'usesNonSdkApi')
    if attr is None:
      attr = doc.createAttributeNS(android_ns, 'android:usesNonSdkApi')
      attr.value = 'true'
      application.setAttributeNode(attr)


def add_use_embedded_dex(doc):
  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    attr = application.getAttributeNodeNS(android_ns, 'useEmbeddedDex')
    if attr is None:
      attr = doc.createAttributeNS(android_ns, 'android:useEmbeddedDex')
      attr.value = 'true'
      application.setAttributeNode(attr)
    elif attr.value != 'true':
      raise RuntimeError('existing attribute mismatches the option of --use-embedded-dex')


def add_extract_native_libs(doc, extract_native_libs):
  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    value = str(extract_native_libs).lower()
    attr = application.getAttributeNodeNS(android_ns, 'extractNativeLibs')
    if attr is None:
      attr = doc.createAttributeNS(android_ns, 'android:extractNativeLibs')
      attr.value = value
      application.setAttributeNode(attr)
    elif attr.value != value:
      raise RuntimeError('existing attribute extractNativeLibs="%s" conflicts with --extract-native-libs="%s"' %
                         (attr.value, value))


def set_has_code_to_false(doc):
  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    attr = application.getAttributeNodeNS(android_ns, 'hasCode')
    if attr is not None:
      # Do nothing if the application already has a hasCode attribute.
      continue
    attr = doc.createAttributeNS(android_ns, 'android:hasCode')
    attr.value = 'false'
    application.setAttributeNode(attr)


def set_test_only_flag_to_true(doc):
  manifest = parse_manifest(doc)
  for application in get_or_create_applications(doc, manifest):
    attr = application.getAttributeNodeNS(android_ns, 'testOnly')
    if attr is not None:
      # Do nothing If the application already has a testOnly attribute.
      continue
    attr = doc.createAttributeNS(android_ns, 'android:testOnly')
    attr.value = 'true'
    application.setAttributeNode(attr)


def set_max_sdk_version(doc, max_sdk_version):
  """Replace the maxSdkVersion attribute value for permission and
  uses-permission tags if the value was originally set to 'current'.
  Used for cts test cases where the maxSdkVersion should equal to
  Build.SDK_INT.

  Args:
    doc: The XML document.  May be modified by this function.
    max_sdk_version: The requested maxSdkVersion attribute.
  """
  manifest = parse_manifest(doc)
  for tag in ['permission', 'uses-permission']:
    children = get_children_with_tag(manifest, tag)
    for child in children:
      max_attr = child.getAttributeNodeNS(android_ns, 'maxSdkVersion')
      if max_attr and max_attr.value == 'current':
        max_attr.value = max_sdk_version


def override_placeholder_version(doc, new_version):
  """Replace the versionCode attribute value if it\'s currently
  set to the placeholder version of 0.

  Args:
    doc: The XML document.  May be modified by this function.
    new_version: The new version to set if versionCode is equal to 0.
  """
  manifest = parse_manifest(doc)
  version = manifest.getAttribute("android:versionCode")
  if version == '0':
    manifest.setAttribute("android:versionCode", new_version)


def main():
  """Program entry point."""
  try:
    args = parse_args()

    doc = minidom.parse(args.input)

    ensure_manifest_android_ns(doc)

    if args.raise_min_sdk_version:
      raise_min_sdk_version(doc, args.min_sdk_version, args.target_sdk_version, args.library)

    if args.max_sdk_version:
      set_max_sdk_version(doc, args.max_sdk_version)

    if args.uses_libraries:
      add_uses_libraries(doc, args.uses_libraries, True)

    if args.optional_uses_libraries:
      add_uses_libraries(doc, args.optional_uses_libraries, False)

    if args.uses_non_sdk_api:
      add_uses_non_sdk_api(doc)

    if args.logging_parent:
      add_logging_parent(doc, args.logging_parent)

    if args.use_embedded_dex:
      add_use_embedded_dex(doc)

    if args.has_no_code:
      set_has_code_to_false(doc)

    if args.test_only:
      set_test_only_flag_to_true(doc)

    if args.extract_native_libs is not None:
      add_extract_native_libs(doc, args.extract_native_libs)

    if args.new_version:
      override_placeholder_version(doc, args.new_version)

    with open(args.output, 'w') as f:
      write_xml(f, doc)

  # pylint: disable=broad-except
  except Exception as err:
    print('error: ' + str(err), file=sys.stderr)
    sys.exit(-1)


if __name__ == '__main__':
  main()
