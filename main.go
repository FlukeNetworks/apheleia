package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/FlukeNetworks/apheleia/nerve"
	"github.com/FlukeNetworks/apheleia/synapse"
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

func createSynapseService(svc Service, zkHosts []string, zkPath string) synapse.Service {
	ssvc := svc.Synapse
	if len(ssvc.DefaultServers) < 1 {
		ssvc.DefaultServers = []synapse.Server{}
	}
	ssvc.Discovery = synapse.Discovery{
		Method: "zookeeper",
		Path: zkPath + svc.GetNodePath(),
		Hosts: zkHosts,
	}
	log.Printf("Setting synapse service port to %d\n", svc.ServicePort)
	ssvc.HAProxy.Port = svc.ServicePort
	return ssvc
}

func createNerveService(svc Service, task taskState, zkHosts []string, zkPath, slaveHost string) nerve.Service {
	return nerve.Service{
		Host: slaveHost,
		Port: task.getPort(svc.PortIndex),
		ReporterType: "zookeeper",
		ZkHosts: zkHosts,
		ZkPath: zkPath + svc.GetNodePath(),
		CheckInterval: svc.Nerve.CheckInterval,
		Checks: svc.Nerve.Checks,
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

func writeJsonFile(file string, data interface{}) error {
	outputFile, err := os.Create(file)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(outputFile)
	if err = encoder.Encode(data); err != nil {
		outputFile.Close()
		return err
	}

	return outputFile.Close()
}

func configureNerve(zkHosts []string, zkPath, slave, nerveCfg, synapseCfg *string, _ []string) {
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

	nerveServices := make(map[string]nerve.Service)
	for _, svc := range node.Services {
		matchingTasks, err := slaveState.getMatchingTasks(svc.Patterns)
		if err != nil {
			log.Fatal(err)
		}
		for _, task := range matchingTasks {
			nsvc := createNerveService(svc, task, zkHosts, *zkPath, slaveState.Hostname)
			nerveServices[svc.Name] = nsvc
		}
	}

	nerveConfig := nerve.Config{
		InstanceId: slaveState.Id,
		Services: nerveServices,
	}

	synapseServices := make(map[string]synapse.Service)
	for _, svc := range node.Services {
		synapseServices[svc.Name] = createSynapseService(svc, zkHosts, *zkPath)
	}

	// Read in the old synapse config
	synapseConfig := func() synapse.Config {
		synapseConfigFile, err := os.Open(*synapseCfg)
		if err != nil {
			log.Fatal(err)
		}
		defer synapseConfigFile.Close()

		var synapseConfig synapse.Config
		decoder := json.NewDecoder(synapseConfigFile)
		if err := decoder.Decode(&synapseConfig); err != nil {
			log.Fatal(err)
		}
		return synapseConfig
	}()
	synapseConfig["services"] = synapseServices

	// Copy the current config to a .old file
	oldNerveCfg := *nerveCfg + ".old"
	if err := copyFile(oldNerveCfg, *nerveCfg); err != nil {
		log.Fatal(err)
	}
	oldSynapseCfg := *synapseCfg + ".old"
	if err := copyFile(oldSynapseCfg, *synapseCfg); err != nil {
		log.Fatal(err)
	}

	// Write the new config
	if err := writeJsonFile(*nerveCfg, &nerveConfig); err != nil {
		log.Fatal(err)
	}
	if err := writeJsonFile(*synapseCfg, synapseConfig); err != nil {
		log.Fatal(err)
	}

	// If the nerve files differ, we need to restart nerve
	shouldRestart, err := filesDiffer(*nerveCfg, oldNerveCfg)
	if err != nil {
		log.Fatal(err)
	}
	if shouldRestart {
		if err := performRestart("NERVE"); err != nil {
			log.Fatal(err)
		}
	}

	// If the synapse files differ, we need to restart synapse
	shouldRestart, err = filesDiffer(*synapseCfg, oldSynapseCfg)
	if err != nil {
		log.Fatal(err)
	}
	if shouldRestart {
		if err := performRestart("SYNAPSE"); err != nil {
			log.Fatal(err)
		}
	}
}

func performRestart(serviceName string) error {
	restartCommand := os.Getenv(fmt.Sprintf("APHELEIA_%s_RESTART_CMD", serviceName))
	cmd := exec.Command("bash", "-c", restartCommand)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
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
		log.Printf("%s should be running on %d\n", svc.Name, svc.ServicePort)
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
	synapseCfg := flag.String("synapseCfg", "synapse.conf.json", "output location for synapse config")
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
		configureNerve(zkHosts, zkPath, slave, nerveCfg, synapseCfg, commandArgs)
	case "updateZk":
		updateZk(zkHosts, zkPath, slave, nerveCfg, synapseCfg, commandArgs)
	default:
		log.Fatal(fmt.Errorf("Unknown command: %s", command))
	}
}
