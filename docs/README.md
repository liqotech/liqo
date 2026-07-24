# Liqo Documentation

The Liqo documentation is available on [doc.liqo.io](https://doc.liqo.io).
The Liqo documentation is built with [Read the Docs](https://readthedocs.org/) and [Awesome Sphinx Theme](https://sphinxawesome.xyz/).

## How to add/update documentation

The documentation content is hosted in the Liqo repository and Read the Docs is in charge of building the documentation for you.
When your PR is merged in master, the documentation will be available on [docs.liqo.io](https://docs.liqo.io).

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

### Links

To create cross-references within the Liqo documentation:

- Add a link to a specific **page**: just add the link to that page, using the full path from within the documentation root folder. For example to create a link towards page *installation/install.md*, you just create ```[your link text](/installation/install.md)```
- Add a link to a specific **section** within a page:

  - first option: similar to the previous case, you can add an explicit link to a sub-section. For example, to create a link towards the *multiple-virtualnodes* anchor within the *advanced/peering/offloading-in-depth.md* page, your link will look like `[your link text](/advanced/peering/offloading-in-depth.md#multiple-virtualnodes)`
  - second option: you need to define a custom anchor in the target page, then you create a link using directly that anchor. For example, you can add the following anchor within a page `(YourNamedAnchor)=`, then you can refer to it within another page by creating this link: `[your link text](YourNamedAnchor)`

## Local testing

In case you would like to browse the documentation website on your local computer, the `Makefile` provides a simple way to do so and make it available at [http://localhost:8080](http://localhost:8080).

```bash
make autoserve
```
