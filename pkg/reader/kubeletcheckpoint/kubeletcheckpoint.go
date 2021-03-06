/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 */

package kubeletcheckpoint

import (
	"fmt"

	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	//	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/errors"
	cpustate "k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/state"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"

	"github.com/fromanirh/numainfo_exporter/pkg/reader/sysfs"
)

// TODO: these are unexported by the kubelet and must be kept in sync manually
const (
	cpuManagerStateFileName = "cpu_manager_state"
)

// CoresAllocation tracks the number of allocated core per NUMA node.
// It is a map whose keys are NUMA node ids and whose values are the number of allocated cores.
type CoresAllocation map[int]int

type Reader struct {
	checkpointManager  checkpointmanager.CheckpointManager
	cpuManagerFileName string
}

func NewReader(stateDir string) (*Reader, error) {
	checkpointManager, err := checkpointmanager.NewCheckpointManager(stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize checkpoint manager: %v", err)
	}
	return &Reader{
		checkpointManager:  checkpointManager,
		cpuManagerFileName: cpuManagerStateFileName,
	}, nil
}

func (r *Reader) SetCPUManagerStateFile(stateFile string) *Reader {
	r.cpuManagerFileName = stateFile
	return r
}

func (r *Reader) GetCoresAllocation(tp sysfs.Topology) (CoresAllocation, error) {
	core2numa := tp.CoreIdToNUMANodeIdMap()

	var err error
	checkpoint := cpustate.NewCPUManagerCheckpoint()

	err = r.checkpointManager.GetCheckpoint(r.cpuManagerFileName, checkpoint)
	if err != nil {
		return nil, err
	}

	coresAlloc := make(CoresAllocation)
	for pod := range checkpoint.Entries {
		for container, cpuString := range checkpoint.Entries[pod] {
			cntCPUSet, err := cpuset.Parse(cpuString)
			if err != nil {
				return nil, fmt.Errorf("could not parse cpuset %q for container %q in pod %q: %v", cpuString, container, pod, err)
			}
			for _, coreId := range cntCPUSet.ToSlice() {
				numaId, ok := core2numa[coreId]
				if !ok {
					klog.Warningf("unknown NUMA node id for core %d in cpuset %q for container id %q", coreId, cpuString, container, pod)
				}
				coresAlloc[numaId]++
			}
		}
	}
	/* 1.17
	for containerID, cpuString := range checkpoint.Entries {
		cntCPUSet, err := cpuset.Parse(cpuString)
		if err != nil {
			return nil, fmt.Errorf("could not parse cpuset %q for container id %q: %v", cpuString, containerID, err)
		}
		for _, coreId := range cntCPUSet.ToSlice() {
			numaId, ok := core2numa[coreId]
			if !ok {
				klog.Warningf("unknown NUMA node id for core %d in cpuset %q for container id %q", coreId, cpuString, containerID)
			}
			coresAlloc[numaId]++
		}
	}
	*/
	return coresAlloc, nil
}
