import semver
import requests

# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Path setup --------------------------------------------------------------

# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
#
# import os
# import sys
# sys.path.insert(0, os.path.abspath('.'))


# -- Project information -----------------------------------------------------

project = 'Liqo'
copyright = '2019-2023, The Liqo Authors'
author = 'The Liqo Authors'


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = [
    'myst_parser',
    'sphinx_copybutton',
    'sphinx_design',
    'sphinx_external_toc',
]

myst_enable_extensions = [
    "substitution",
]

# Add any paths that contain templates here, relative to this directory.
templates_path = ['_templates']

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ['_build', '_legacy', 'Thumbs.db', '.DS_Store', 'README.md', 'requirements.txt']

linkcheck_ignore = [
    r'http://localhost:\d+/?',  # Localhost links
    'https://github.com/virtual-kubelet/virtual-kubelet#liqo-provider'  # This anchor is not found by the linter, but it works
]


# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_theme = 'sphinx_rtd_theme'

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
html_static_path = ['_static']

# These paths are either relative to html_static_path or fully qualified paths (eg. https://...)
html_css_files = ['css/custom.css']

html_title = 'Liqo'

# Configure the logo shown above the search bar.
html_logo = "_static/images/common/liqo-logo.svg"

# Configure the favicon.
html_favicon = "_static/images/common/favicon.svg"

html_theme_options = {
    # 'analytics_id': 'G-XXXXXXXXXX',  #  Provided by Google in your dashboard
    # 'analytics_anonymize_ip': False,
    'logo_only': False,
    'display_version': True,
    'prev_next_buttons_location': 'bottom',
    'style_external_links': False,
    'vcs_pageview_mode': '',
    # 'style_nav_header_background': 'white',
    # Toc options
    'collapse_navigation': True,
    'sticky_navigation': True,
    'navigation_depth': 3,
    'includehidden': True,
    'titles_only': False
}

# External TOC options
external_toc_path = "_toc.yml"
external_toc_exclude_missing = True

# __get_download_url returns the download URL for the given file, for the latest or the given GitHub release.
def __get_download_url(file: str) -> str:
    if 'current_version' not in html_context or not __is_sem_version(html_context['current_version']):
        return f"https://github.com/liqotech/liqo/releases/latest/download/{file}"
    else:
        return f"https://github.com/liqotech/liqo/releases/download/{html_context['current_version']}/{file}"

# generate_clone_example generates the clone and checkout code for the given example.
def generate_clone_example(example_name: str) -> str:
    version = html_context['github_version']
    if 'current_version' in html_context and html_context['current_version'] == 'stable':
        x = requests.get('https://api.github.com/repos/liqotech/liqo/releases/latest')
        version = x.json()['tag_name']
    return f"```bash\n\
git clone https://github.com/liqotech/liqo.git\n\
cd liqo\n\
git checkout {version}\n\
cd examples/{example_name}\n\
./setup.sh\n\
```\n"

# generate_clone_example_tf generates the clone and checkout code for the given example.
def generate_clone_example_tf(example_name: str) -> str:
    version = html_context['github_version']
    if 'current_version' in html_context and html_context['current_version'] == 'stable':
        x = requests.get('https://api.github.com/repos/liqotech/liqo/releases/latest')
        version = x.json()['tag_name']
    return f"```bash\n\
git clone https://github.com/liqotech/liqo.git\n\
cd liqo\n\
git checkout {version}\n\
cd examples/{example_name}\n\
terraform init\n\
terraform apply\n\
```\n"

# generate_liqoctl_install generates the liqoctl installation instruction for the given platform and architecture.
def generate_liqoctl_install(platform: str, arch: str) -> str:
    if platform == 'windows':
        return f"```bash\n\
curl --fail -LSO \"{__get_download_url('liqoctl-windows-amd64')}\"\n\
```\n"
    elif platform == 'darwin':
        file = __get_download_url(f"liqoctl-darwin-{arch}.tar.gz")
        return f"```bash\n\
curl --fail -LS \"{file}\" | tar -xz\n\
chmod +x liqoctl\n\
sudo mv liqoctl /usr/local/bin/liqoctl\n\
```\n"
    elif platform == 'linux':
        file = __get_download_url(f"liqoctl-linux-{arch}.tar.gz")
        return f"```bash\n\
curl --fail -LS \"{file}\" | tar -xz\n\
sudo install -o root -g root -m 0755 liqoctl /usr/local/bin/liqoctl\n\
```\n"

# __is_sem_version returns True if the given string is a semantic version or the 'stable' string.
def __is_sem_version(version: str) -> bool:
    return semver.VersionInfo.isvalid(version.lstrip("v"))

# generate_liqoctl_version_warning generates the liqoctl version warning when the documentation is not for a released version.
def generate_liqoctl_version_warning() -> str:
    if 'current_version' not in html_context or (not __is_sem_version(html_context['current_version']) and html_context['current_version'] != 'stable'):
        return "```{warning}\n\
The following instructions will guide you through the installation of the **latest stable version of *liqoctl***, which might exhibit a different behavior compared to the one shown in the rest of this documentation. If you want to use the latest *liqoctl* version, [build it from source](InstallationLiqoctlFromSource).\n\
```\n"
    return ""

html_context = {
    'generate_clone_example': generate_clone_example,
    'generate_clone_example_tf': generate_clone_example_tf,
    'generate_liqoctl_install': generate_liqoctl_install,
    'generate_liqoctl_version_warning': generate_liqoctl_version_warning,
    'github_repo': 'liqo',
    'github_version': 'master',
    'display_github': True,
    'commit': 'abcdefgh',
}
