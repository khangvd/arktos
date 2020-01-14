/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllerframework

import (
	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"sort"
)

// Sort Controller Instances by controller key
func SortControllerInstancesByKeyAndConvertToLocal(controllerInstanceMap map[string]v1.ControllerInstance) []controllerInstanceLocal {
	// copy map
	var sortedControllerInstancesLocal []controllerInstanceLocal
	for _, controllerInstance := range controllerInstanceMap {
		instance := controllerInstanceLocal{
			instanceName:  controllerInstance.Name,
			controllerKey: controllerInstance.ControllerKey,
			workloadNum:   controllerInstance.WorkloadNum,
			isLocked:      controllerInstance.IsLocked,
		}
		sortedControllerInstancesLocal = append(sortedControllerInstancesLocal, instance)
	}

	sort.Slice(sortedControllerInstancesLocal, func(i, j int) bool {
		return sortedControllerInstancesLocal[i].controllerKey < sortedControllerInstancesLocal[j].controllerKey
	})

	if len(sortedControllerInstancesLocal) > 0 {
		sortedControllerInstancesLocal[0].lowerboundKey = 0
	}

	for i := 1; i < len(sortedControllerInstancesLocal); i++ {
		sortedControllerInstancesLocal[i].lowerboundKey = sortedControllerInstancesLocal[i-1].controllerKey
	}

	return sortedControllerInstancesLocal
}

// The following are test util. Put here so that can be used in other packages' tests

var alwaysReady = func() bool { return true }
var notifyTimes int

func mockNotifyHander(controllerInstance *v1.ControllerInstance) {
	notifyTimes++
	return
}
func mockCheckInstanceHander() {
	return
}

func CreateTestControllerInstanceManager(stopCh chan struct{}, updateCh chan string) (*ControllerInstanceManager, informers.SharedInformerFactory) {
	client := fake.NewSimpleClientset()
	informers := informers.NewSharedInformerFactory(client, 0)

	cim := NewControllerInstanceManager(informers.Core().V1().ControllerInstances(), client, updateCh)
	go cim.Run(stopCh)

	cim.controllerListerSynced = alwaysReady
	cim.notifyHandler = mockNotifyHander
	checkInstanceHandler = mockCheckInstanceHander
	return GetControllerInstanceManager(), informers
}