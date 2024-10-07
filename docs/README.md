# Liqo Documentation

The Liqo documentation is available on [doc.liqo.io](https://doc.liqo.io).
The Liqo documentation is built with [Read the Docs](https://readthedocs.org/) and [Awesome Sphinx Theme](https://sphinxawesome.xyz/).

## How to add/update documentation

The documentation content is hosted in the Liqo repository and Read the Docs is in charge of building the documentation for you.
When your PR is merged in master, the documentation will be available on [doc.liqo.io](https://doc.liqo.io).

### Pages

Documentation pages should be put in the [docs/ folder](docs/).
This directory reflects the whole structure of the documentation website.
Adding a subdirectory corresponds to a new sub-chapter.
For example:

```text
user
├── install 
│   ├── index.md       <-- /user/install/
architecture
developers
index.md              <-- /
```

### Images

Images should be put in the [docs/_static/images folder](docs/_static/images).

## Local testing

In case you want to browse the documentation website on your local computer, the Makefile provides a simple way to do so and make it available at [http://localhost:8080](http://localhost:8080).

```bash
make serve
```
