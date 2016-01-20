# apheleia
[![Build Status](https://travis-ci.org/FlukeNetworks/apheleia.svg?branch=master)](https://travis-ci.org/FlukeNetworks/apheleia)
[![License](https://img.shields.io/badge/license-GPLv3-blue.svg)](https://github.com/FlukeNetworks/apheleia/blob/master/LICENSE)
[![Stories in Ready](https://badge.waffle.io/FlukeNetworks/apheleia.svg?label=ready&title=ready)](http://waffle.io/FlukeNetworks/apheleia)

`apheleia` is a reconfiguration utility for [nerve](https://github.com/airbnb/nerve) similar to Yelp's [nerve-tools](https://github.com/Yelp/nerve-tools) `configure_nerve`, but with its data based in zookeeper and not on disk.

# Usage

```bash
apheleia [options] command [commandArgs...]
```

## Commands

The following commands are available to the apheleia CLI.

| Command | Description | Arguments |
| ------- | ----------- | --------- |
| `configureNerve` | Generates a nerve configuration file and restarts nerve (if the configuration changed) | None |
| `updateZk` | Updates zookeeper with the given YAML files describing services | list of yml service description files |

## Flags

The following flags may be used to configure apheleia.

| Flag | Example | Description |
| ---- | ------- | ----------- |
| `-zk hosts` | `-zk localhost:2181,otherHost:2181` | comma separated list of zookeeper hosts and ports |
| `-zkPath path` | `-zkPath /apheleia` | zookeeper path of the apheleia node |
| `-slave slaveURL` | `-slave http://localhost:5051` | base URL of the mesos slave API with which to communicate |
| `-nerveCfg nerveCfgFile` | `-nergeCfg /opt/smartstack/nerve/nerve.conf` | nerve configuration file to manage |

## Environment Variables

The following environment variables may be used to configure apheleia

| Variable | Description |
| -------- | ----------- |
| `APHELEIA_NERVE_RESTART_CMD` | bash command used to restart nerve after a configuration update |

## Examples

### Configure nerve (systemd)

The following example configures nerve running under systemd for an environment where zookeeper and the mesos slave api are running locally on standard ports

```bash
#!/usr/bin/env bash

export APHELEIA_NERVE_RESTART_CMD="systemctl restart nerve"
apheleia \
	-zk localhost:2181 \
	-zkPath /apheleia \
	-slave http://localhost:5051 \
	-nerveCfg /opt/smartstack/nerve/nerve.conf \
	configureNerve
```

### Update zookeeper data from dir of yaml files

The following example updates zookeeper data for apheleia from yaml service definition files in the `./services` directory

Like the previous example, it assumes an environment where zookeeper is running locally on the standard port.

```bash
#!/usr/bin/env bash

service_dir=services
service_files=$(find "$service_dir" -type f -name '*.yml')

apheleia \
	-zk localhost:2181 \
	-zkPath /apheleia \
	updateZk $service_files
```

# License

Licensed under GPLv3. See [`LICENSE`](https://github.com/FlukeNetworks/apheleia/blob/master/LICENSE) file.
