import semver
import requests

from dataclasses import asdict

from sphinxawesome_theme import ThemeOptions
from sphinxawesome_theme.postprocess import Icons

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
copyright = '2019-2025, The Liqo Authors'
author = 'The Liqo Authors'


# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = [
    'sphinxawesome_theme',
    'myst_parser',
    'sphinx_design',
    'sphinx_external_toc',
]

myst_enable_extensions = [
    "substitution",
]
# Enable slug generation for headings to reference them in markdown links
myst_heading_anchors = 4

# Add any paths that contain templates here, relative to this directory.
templates_path = ['_templates']

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ['_build', '_legacy', 'Thumbs.db', '.DS_Store', 'README.md', 'requirements.txt']

linkcheck_ignore = [
    r'http://localhost:\d+/?',  # Localhost links
    'https://github.com/virtual-kubelet/virtual-kubelet#liqo-provider',  # This anchor is not found by the linter, but it works
    'https://www.wireguard.com/', # knwon recurrent error 500
    'https://medium.com/', # often rate-limited
    'https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/1645-multi-cluster-services-api#service-types',
    'https://ieeexplore.ieee.org',
]


# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_permalinks_icon = '<span class="fa fa-link text-sm"></span>'
html_theme = 'sphinxawesome_theme'
pygments_style = "colorful"
pygments_style_dark = "dracula"

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
html_static_path = ['_static']

# These paths are either relative to html_static_path or fully qualified paths (eg. https://...)
html_css_files = [
    'css/custom.css',
    'https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.1.1/css/all.min.css'
]

html_title = 'Liqo'

# Configure the logo shown above the search bar.
html_logo = "_static/images/common/liqo-logotype.svg"

# Configure the favicon.
html_favicon = "_static/images/common/favicon.svg"

theme_options = ThemeOptions(
    extra_header_link_icons={
        "the GitHub repository": {
            "link": "https://github.com/liqotech/liqo",
            "icon": (
                '<svg height="26px" style="margin-top:-2px;display:inline" '
                'viewBox="0 0 45 44" '
                'fill="currentColor" xmlns="http://www.w3.org/2000/svg">'
                '<path fill-rule="evenodd" clip-rule="evenodd" '
                'd="M22.477.927C10.485.927.76 10.65.76 22.647c0 9.596 6.223 17.736 '
                "14.853 20.608 1.087.2 1.483-.47 1.483-1.047 "
                "0-.516-.019-1.881-.03-3.693-6.04 "
                "1.312-7.315-2.912-7.315-2.912-.988-2.51-2.412-3.178-2.412-3.178-1.972-1.346.149-1.32.149-1.32 "
                "2.18.154 3.327 2.24 3.327 2.24 1.937 3.318 5.084 2.36 6.321 "
                "1.803.197-1.403.759-2.36 "
                "1.379-2.903-4.823-.548-9.894-2.412-9.894-10.734 "
                "0-2.37.847-4.31 2.236-5.828-.224-.55-.969-2.759.214-5.748 0 0 "
                "1.822-.584 5.972 2.226 "
                "1.732-.482 3.59-.722 5.437-.732 1.845.01 3.703.25 5.437.732 "
                "4.147-2.81 5.967-2.226 "
                "5.967-2.226 1.185 2.99.44 5.198.217 5.748 1.392 1.517 2.232 3.457 "
                "2.232 5.828 0 "
                "8.344-5.078 10.18-9.916 10.717.779.67 1.474 1.996 1.474 4.021 0 "
                "2.904-.027 5.247-.027 "
                "5.96 0 .58.392 1.256 1.493 1.044C37.981 40.375 44.2 32.24 44.2 "
                '22.647c0-11.996-9.726-21.72-21.722-21.72" '
                'fill="currentColor"/></svg>'
            ),
        },
    },
)

html_theme_options = {
    **asdict(theme_options)
}

# External TOC options
external_toc_path = "_toc.yml"
external_toc_exclude_missing = True

# __get_download_url returns the download URL for the given file, for the latest or the given GitHub release.
def __get_download_url(file: str) -> str:
    version = generate_semantic_version()
    return f"https://github.com/liqotech/liqo/releases/download/{version}/{file}"

# generate_semantic_version generates the semantic version string for the current page.
# latest: return the latest release (get the last release tag from GitHub API)
# stable: return the latest release (get the last release tag from GitHub API)
# tag: return the tag specified in the html_context[github_version]
def generate_semantic_version() -> str:
    version = html_context['github_version'] if 'github_version' in html_context else 'master'
    if 'current_version' in html_context and html_context['current_version'] == 'stable':
        x = requests.get('https://api.github.com/repos/liqotech/liqo/releases/latest')
        version = x.json()['tag_name']
    if version == 'master':
        x = requests.get('https://api.github.com/repos/liqotech/liqo/releases/latest')
        version = x.json()['tag_name']
    return version

# generate_version generates the version string for the current page.
# latest: return master
# stable: return the latest release (get the last release tag from GitHub API)
# tag: return the tag specified in the html_context[github_version]
def generate_version() -> str:
    version = html_context['github_version'] if 'github_version' in html_context else 'master'
    if 'current_version' in html_context and html_context['current_version'] == 'stable':
        x = requests.get('https://api.github.com/repos/liqotech/liqo/releases/latest')
        version = x.json()['tag_name']
    return version

# generate_link_to_repo generates a link to the repository for the given file, according to the current version of the documentation.
def generate_link_to_repo(text: str, file_path: str) -> str:
    version = generate_version()
    return f"[{text}](https://github.com/liqotech/liqo/blob/{version}/{file_path})"

# generate_clone_example generates the clone and checkout code for the given example.
def generate_clone_example(example_name: str) -> str:
    version = generate_version()
    return f"```bash\n\
git clone https://github.com/liqotech/liqo.git\n\
cd liqo\n\
git checkout {version}\n\
cd examples/{example_name}\n\
./setup.sh\n\
```\n"

# generate_clone_example_tf generates the clone and checkout code for the given example.
def generate_clone_example_tf(example_name: str) -> str:
    version = generate_version()
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

def generate_helm_install() -> str:
    version=generate_semantic_version()
    return f"```bash\n\
helm install liqo liqo/liqo --namespace liqo --version {version}\n\
    --values <path-to-values-file> --create-namespace\n\
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
    'generate_link_to_repo': generate_link_to_repo,
    'generate_helm_install': generate_helm_install,
    'generate_liqoctl_version_warning': generate_liqoctl_version_warning,
    'github_repo': 'liqo',
    'display_github': True,
}

suppress_warnings = ["config.cache"]
