# Cloudwork

Cloudwork is a bare-bones automation tool for quickly spawning machines in the cloud and distributing workloads across them.

> [!WARNING]  
> Screen recording is outdated.

https://github.com/3rd/cloudwork/assets/59587503/391373cd-7b69-4b21-85c9-2a8d35cd3e3f

## Installation

Requirements:

- `ssh`
- `rsync`

```bash
go install github.com/3rd/cloudwork@latest
```

## Setup

First, you'll need to create a `cloudwork.yml` configuration file.
\
The configuration file is a YAML file that looks like this:

```yaml
workers:
  - host: worker1
  - host: worker2
  - host: worker3

scripts:
  setup: |
    # This runs when you do `cloudwork setup` if the worker wasn't already setup or if the script changed.
    apt update -y && apt upgrade -y
    apt install -y apt-transport-https ca-certificates curl software-properties-common gcc clang make build-essential libssl-dev libffi-dev libpcap-dev
    apt install -y neovim
    curl -s https://get.docker.com/ | sh
    usermod -aG docker $USER
    mkdir /app
    upload Dockerfile /app

  default: |
    # This runs on each worker when you do `cloudwork run`.
    upload-input /tmp/worker/input/
    docker build -t work /app
    docker run -v /tmp/worker/input:/input -v /tmp/worker/output:/output -it work
    download-output /tmp/worker/output/
```

Use `cloudwork bootstrap` to create the input/output directory structure for the configured workers.
\
After running this command, you will have a "workers" directory with the following structure:

```
|-- workers/
|   |-- worker1/
|   |   |-- input/*
|   |   |-- output/*
|   |-- worker2/
|   |   |-- input/*
|   |   |-- output/*
|   ...
|-- cloudwork.yml
```

In the example configuration, `setup` and `run` are scripts that will run on workers.
\
There are a few special commands that you can use in scripts:

- `upload-input <remote path>` - Uploads `./workers/<worker/input/*` to the worker at `<remote path>`.
- `download-output <remote path>` - Downloads `<remote path>` to `./workers/<worker/output/`.
- `upload <local path> <remote path>` - Uploads localhost:`<local path>` to remote: `<remote path>`.
- `download <remote path> <local path>` - Downloads remote: `<remote path>` to localhost:`<local path>`.

> [!WARNING]
> Upload and download operations are extracted from the script and don't run when you'd expect.
> Uploads are done before the script is run, and downloads are done after the script is run.

> [!TIP]
> The paths used in the upload/download commens are pass as-they-are to `rsync`, remember to add a trailing slash if you want to upload/download the contents of a directory.

## Usage

- `cloudwork bootstrap` - Creates the input/output directory structure for the configured workers.
- `cloudwork run [script-name]` - Runs the specified script on all workers. If no script name is provided, it runs the "default" script.
- `cloudwork run ./script.sh` - Runs a script (file) on all workers.
- `cloudwork exec "command"` - Executes a command on all workers.
- `couldwork -host <host> <command>` - idem, but only for the specified `<host>`.

## Examples

### Subdomain enumeration with puredns

```yaml
workers:
  - host: worker1
  - host: worker2
  - host: worker3

scripts:
  setup: |
    apt update -y && apt upgrade -y
    apt install -y apt-transport-https ca-certificates curl software-properties-common gcc clang make build-essential libssl-dev libffi-dev libpcap-dev
    apt install -y golang-go

    mkdir -p "$HOME/go"
    export GOPATH="$HOME/go"
    export PATH="$GOPATH/bin:$PATH"
    grep -q "export GOPATH" ~/.profile || {
        echo "export GOPATH=$HOME/go" >>~/.profile
        echo "export PATH=\$GOPATH/bin:\$PATH" >>~/.profile
    }

    git clone https://github.com/blechschmidt/massdns.git
    cd massdns
    make
    make install

    go install github.com/d3mondev/puredns/v2@latest

    mkdir -p /worker
    upload sample-wordlist.txt /worker
    upload sample-resolvers.txt /worker

  default: |
    puredns bruteforce \
        /worker/sample-wordlist.txt \
        -t 100 -rate-limit 500 \
        -r /worker/sample-resolvers.txt \
        -d /tmp/worker/input/domains.txt \
        -w /tmp/worker/output/results.txt \
        --write-wildcards /tmp/worker/output/wildcards.txt
```
