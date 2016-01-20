package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type slaveState struct {
	Id string `json:"id"`
	Hostname string `json:"hostname"`
	Frameworks []frameworkState `json:"frameworks"`
}

type frameworkState struct {
	Executors []executorState `json:"executors"`
}

type executorState struct {
	Id string `json:"id"`
	Tasks []taskState `json:"tasks"`
}

type taskState struct {
	Name string `json:"name"`
	Resources map[string]interface{} `json:"resources"`
}

func getSlaveState(slaveUrl string) (*slaveState, error) {
	resp, err := http.Get(slaveUrl + "/slave(1)/state")
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	var s slaveState
	if err = decoder.Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (s *slaveState) getMatchingTasks(patterns ServicePatterns) ([]taskState, error) {
	var matchingTasks = make([]taskState, 0)

	executorPattern, err := regexp.Compile(patterns.Executor)
	if err != nil {
		return nil, err
	}
	taskPattern, err := regexp.Compile(patterns.Task)

	for _, fw := range s.Frameworks {
		for _, executor := range fw.Executors {
			if executorPattern.MatchString(executor.Id) {
				for _, task := range executor.Tasks {
					if taskPattern.MatchString(task.Name) {
						matchingTasks = append(matchingTasks, task)
					}
				}
			}
		}
	}

	return matchingTasks, nil
}

func (task *taskState) getPort(index int) int {
	portsString := task.Resources["ports"].(string)
	portsString = strings.Trim(portsString, "[]")

	portRangeStrs := strings.Split(portsString, ",")
	portRanges := make([][]string, len(portRangeStrs))
	for idx, portRangeStr := range portRangeStrs {
		portRanges[idx] = strings.Split(portRangeStr, "-")
	}

	i, err := strconv.Atoi(portRanges[index][0])
	if err != nil {
		panic("Invalid port from mesos")
	}

	return i
}
