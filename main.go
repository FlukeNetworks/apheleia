package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/FlukeNetworks/apheleia/nerve"
	"github.com/samuel/go-zookeeper/zk"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

func createNerveService(svc Service, task taskState, zkHosts []string, zkPath, slaveHost string) nerve.Service {
	return nerve.Service{
		Host: slaveHost,
		Port: task.getPort(svc.PortIndex),
		ReporterType: "zookeeper",
		ZkHosts: zkHosts,
		ZkPath: zkPath + "/" + svc.Name,
		CheckInterval: svc.CheckInterval,
		Checks: svc.Checks,
	}
}

func configureNerve(zkHosts []string, zkPath, slave, slaveHost, nerveCfg *string, _ []string) {
	slaveState, err := getSlaveState(*slave)
	if err != nil {
		log.Fatal(err)
	}

	zkConn, _, err := zk.Connect(zkHosts, 10 * time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	defer zkConn.Close()

	// Get the service information from Zookeeper
	nodeBytes, _, err := zkConn.Get(*zkPath)
	if err != nil {
		log.Fatal(err)
	}
	var node ApheleiaNode
	if err = json.Unmarshal(nodeBytes, &node); err != nil {
		log.Fatal(err)
	}

	nerveServices := make([]nerve.Service, 0)
	for _, svc := range node.Services {
		matchingTasks, err := slaveState.getMatchingTasks(svc.Patterns)
		if err != nil {
			log.Fatal(err)
		}
		for _, task := range matchingTasks {
			nsvc := createNerveService(svc, task, zkHosts, *zkPath, *slaveHost)
			nerveServices = append(nerveServices, nsvc)
		}
	}

	nerveConfig := nerve.Config{
		InstanceId: *slaveHost,
		Services: nerveServices,
	}

	outputFile, err := os.Create(*nerveCfg)
	if err != nil {
		log.Fatal(err)
	}
	defer outputFile.Close()

	encoder := json.NewEncoder(outputFile)
	if err = encoder.Encode(&nerveConfig); err != nil {
		log.Fatal(err)
	}
}

func updateZk(zkHosts []string, zkPath, slave, _, _ *string, serviceFiles []string) {
	services := make([]Service, 0)
	for _, serviceFile := range serviceFiles {
		fileBytes, err := ioutil.ReadFile(serviceFile)
		if err != nil {
			log.Fatal(err)
		}

		var svc Service
		if err = yaml.Unmarshal(fileBytes, &svc); err != nil {
			log.Fatal(err)
		}
		services = append(services, svc)
	}

	node := ApheleiaNode{
		Services: services,
	}
	nodeBytes, err := json.Marshal(&node)
	if err != nil {
		log.Fatal(err)
	}

	zkConn, _, err := zk.Connect(zkHosts, 10 * time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	defer zkConn.Close()

	nodeExists, nodeStats, err := zkConn.Exists(*zkPath)
	if err != nil {
		log.Fatal(err)
	}

	if nodeExists {
		_, err = zkConn.Set(*zkPath, nodeBytes, nodeStats.Version)
	} else {
		_, err = zkConn.Create(*zkPath, nodeBytes, 0, zk.WorldACL(zk.PermAll))
	}
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	zkArg := flag.String("zk", "", "zookeeper hosts string")
	zkPath := flag.String("zkPath", "/apheleia", "zookeeper path for this service keyspace")
	slave := flag.String("slave", "http://localhost:5051", "base URI for mesos slave API")
	slaveHost := flag.String("slaveHost", "localhost", "hostname for slave when registering services")
	nerveCfg := flag.String("nerveCfg", "nerve.conf.json", "output location for nerve config")
	flag.Parse()
	zkHosts := strings.Split(*zkArg, ",")

	freeArgs := flag.Args()
	if len(freeArgs) < 1 {
		log.Fatal(errors.New("You must supply a command"))
	}
	command := freeArgs[0]
	commandArgs := freeArgs[1:]

	switch command {
	case "configureNerve":
		configureNerve(zkHosts, zkPath, slave, slaveHost, nerveCfg, commandArgs)
	case "updateZk":
		updateZk(zkHosts, zkPath, slave, slaveHost, nerveCfg, commandArgs)
	default:
		log.Fatal(fmt.Errorf("Unknown command: %s", command))
	}
}
