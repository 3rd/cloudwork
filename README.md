# Cloudwork

Cloudwork is a bare-bones automation tool for quickly spawning machines in the cloud and distributing workloads across them.

## Installation

```bash
go install github.com/3rd/cloudwork@latest
```

## Setup

You'll need to create a `cloudwork.yml` configuration and a directory structure like this:

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
Workers are controlled over SSH, make `ssh workerX` work before you start.
\
Whatever is in the `$worker/input` directory will be uploaded to the worker at `/tmp/worker/input`.
\
Whatever the worker writes to `/tmp/worker/output` will be downloaded to the local `$worker/output` directory.
\
The configuration file is a YAML file that looks like this:

```yaml
workers:
    - host: worker1
    - host: worker2
    - host: worker3

setup: |
    # This runs when you do `cloudwork setup` if the worker wasn't already setup or if the script changed.
    apt update -y && apt upgrade -y
    apt install -y apt-transport-https ca-certificates curl software-properties-common gcc clang make build-essential libssl-dev libffi-dev libpcap-dev
    apt install -y neovim
    curl -s https://get.docker.com/ | sh
    usermod -aG docker $USER
    mkdir /app
    upload Dockerfile /app

run: |
    # This runs on each worker when you do `cloudwork run`.
    docker build -t work /app
    docker run -v /tmp/worker/input:/input -v /tmp/worker/output:/output -it work
    
```

There are a few special commands that you can use in the `setup` and `run` scripts:

- `upload <local path> <remote path>` - Uploads things to the remote machine, all uploads are done before the script is executed.
- `download <remote path> <local path>` - Downloads things from the remote machine, all downloads are done after the script is executed.

## Usage

- `cloudwork bootstrap` - Creates the input/output directory structure for the configured workers.
- `cloudwork setup` - Runs the `setup` script on each worker.
- `cloudwork run` - Uploads inputs, executes the `run` script on all workers, and downloads outputs.
- `couldwork -host <host> <command>` - idem, but only for the specified host.

## Examples

### Subdomain enumeration with puredns

```yaml
workers:
    - host: worker1
    - host: worker2
    - host: worker3

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

run: |
    puredns bruteforce \
        /worker/sample-wordlist.txt \
        -t 100 -rate-limit 500 \
        -r /worker/sample-resolvers.txt \
        -d /tmp/worker/input/domains.txt \
        -w /tmp/worker/output/results.txt \
        --write-wildcards /tmp/worker/output/wildcards.txt
```

