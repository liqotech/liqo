# Liqo Documentation

The Liqo documentation is available on [doc.liqo.io](https://doc.liqo.io).  The Liqo documentation is 
built on [Hugo](https://gohugo.io/) and the [Learn theme](https://themes.gohugo.io/hugo-theme-learn/). 

## How to add/update documentation

The documentation content are hosted in Liqo repository and a dedicated pipeline is in charge of building the documentation
for you. When your PR is merged in master, the documentation will be available on [doc.liqo.io](https://doc.liqo.io).
A similar pipeline exists which updates an internal pre-production site each time a new change for the documentation is committed in another branch (not master).

### Pages

Documentation pages should be put in [docs/pages](docs/pages). This directory reflects the whole structure of the documentation
website. In fact, adding a subdirectory corresponds to a new sub-chapter. For example: 

```
pages
├── User
│   ├── Install 
│   │   ├── _index.md       <-- /user/install/
├── Architecture
├── Developers
├── _index.md                       <-- /
```

Each _index.md represent the index page of each subchapter. 

### Images

Images should be put in [docs/images](docs/images). When referencing an image from a documentation page (i.e. an MD file)
you should use an absolute link taking docs as root. For example, if you add an image to *docs/images/install/test.png*,
the link will be ```![](/images/install/test.png)```. Obviously, you should pay attention to put the first slash "/".

