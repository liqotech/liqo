# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXOPTS    ?= -W -n --keep-going
SPHINXAUTOOPTS    ?= -W -n
SPHINXBUILD   ?= sphinx-build
SPHINXAUTOBUILD ?= sphinx-autobuild
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
	pip3 install -r requirements.txt

serve: install html
	docker run --rm -it -v $(PWD)/_build/html:/usr/share/nginx/html -p 8080:80 nginx

autoserve: install
	@$(SPHINXAUTOBUILD) "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXAUTOOPTS) --port 8081 $(O)

lint:
	markdownlint lint -c ../.markdownlint.yml **/*.md
