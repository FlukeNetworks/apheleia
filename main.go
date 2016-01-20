package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/FlukeNetworks/apheleia/nerve"
	"github.com/samuel/go-zookeeper/zk"
	yaml "gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const ChecksumSize = md5.Size

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

func copyFile(dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}

	return d.Close()
}

func fileChecksum(filename string) ([ChecksumSize]byte, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return [ChecksumSize]byte{}, err
	}
	return md5.Sum(fileBytes), nil
}

func filesDiffer(first, second string) (bool, error) {
	firstSum, err := fileChecksum(first)
	if err != nil {
		return false, err
	}
	secondSum, err := fileChecksum(second)
	if err != nil {
		return false, err
	}
	return (firstSum != secondSum), nil
}

func configureNerve(zkHosts []string, zkPath, slave, nerveCfg *string, _ []string) {
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
			nsvc := createNerveService(svc, task, zkHosts, *zkPath, slaveState.Hostname)
			nerveServices = append(nerveServices, nsvc)
		}
	}

	nerveConfig := nerve.Config{
		InstanceId: slaveState.Id,
		Services: nerveServices,
	}

	// Copy the current config to a .old file
	oldNerveCfg := *nerveCfg + ".old"
	if err := copyFile(oldNerveCfg, *nerveCfg); err != nil {
		log.Fatal(err)
	}

	// Write the new nerve config
	func() {
		outputFile, err := os.Create(*nerveCfg)
		if err != nil {
			log.Fatal(err)
		}
		defer outputFile.Close()

		encoder := json.NewEncoder(outputFile)
		if err = encoder.Encode(&nerveConfig); err != nil {
			log.Fatal(err)
		}
	}()

	// If the files differ, we need to restart nerve
	shouldRestart, err := filesDiffer(*nerveCfg, oldNerveCfg)
	if err != nil {
		log.Fatal(err)
	}
	if shouldRestart {
		nerveRestartCommand := os.Getenv("NERVE_RESTART_CMD")
		cmd := exec.Command("bash", "-c", nerveRestartCommand)
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}
}

func updateZk(zkHosts []string, zkPath, slave, _ *string, serviceFiles []string) {
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
		configureNerve(zkHosts, zkPath, slave, nerveCfg, commandArgs)
	case "updateZk":
		updateZk(zkHosts, zkPath, slave, nerveCfg, commandArgs)
	default:
		log.Fatal(fmt.Errorf("Unknown command: %s", command))
	}
}
