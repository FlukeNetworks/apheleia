#!/usr/bin/env bash
config=$(realpath $NERVE_CFG)

cp $config $config.old
apheleia -zk "$ZK_HOSTS" -zkPath "$ZK_PATH" -slave "http://$SLAVE_HOST:5051" -slaveHost "$SLAVE_HOST" -nerveCfg "$config" configureNerve
changed=$(diff "$config" "$config.old")
if [ ! -z "$changed" ]; then
	$NERVE_RESTART_COMMAND
fi
