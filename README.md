# Weg

A modern CLI tool for managing Frappe deployments with blazing fast performance.

## What is Weg?

Weg (वेग - Marathi for "fast" or German for "way") is a command-line interface tool designed to provide a modern developer experience for managing Frappe deployments. It handles end-to-end dependencies, including specific system dependency versions, through Nix/devbox integration and offers blazingly fast concurrent application resolution.

## Features

- 🚀 Blazing fast performance
- 🔧 End-to-end dependency management
- 🎯 System dependency version control via Nix/devbox
- ⚡ Concurrent application resolution
- 🛠️ Modern developer experience

## Installation

Download `weg` binary or compile from source.

## Usage

### Creating a new Bench

Pass the versions of the apps you want

```bash
weg create $HOME/.weg-bench-$(date +%s) --apps '[{"Url": "https://github.com/The-Commit-Company/raven"}, {"Url": "https://github.com/frappe/erpnext", "Branch": "v15.64.1"}]' -v 'v15.69.3'
```

>![NOTE]
> Branch takes vaid git tags and branches as input and handles system dependency management automatically.


## License
