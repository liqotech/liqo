# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXOPTS    ?= -W -n --keep-going
SPHINXAUTOOPTS    ?= -W -n
VENV ?= .venv
PYTHON ?= $(VENV)/bin/python
PIP ?= $(VENV)/bin/pip
SPHINXBUILD   ?= $(VENV)/bin/sphinx-build
SPHINXAUTOBUILD ?= $(VENV)/bin/sphinx-autobuild
SOURCEDIR     = .
BUILDDIR      = _build

# Put it first so that "make" without argument is like "make help".
help:
	@$(SPHINXBUILD) -M help "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

.PHONY: help Makefile

# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINXOPTS).
%: Makefile
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

install:
	@if [ ! -d "$(VENV)" ]; then python3 -m venv $(VENV); fi
	@$(PIP) install --upgrade pip setuptools wheel
	@$(PIP) install -r requirements.txt

serve: install html
	@$(PYTHON) -m http.server 8080 --directory $(BUILDDIR)/html

autoserve: install
	@$(SPHINXAUTOBUILD) "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXAUTOOPTS) --port 8081 $(O)

lint:
	markdownlint lint -c ../.markdownlint.yml -i usage/liqoctl **/*.md

clean-venv:
	@rm -rf $(VENV)
