/*
Copyright 2020 The Kubernetes Authors.

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

package scheduling

import (
	"context"
	"fmt"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/allocator"
	"github.com/yaoice/meliodas/pkg/ipam/backend/neutron"
	"github.com/gophercloud/gophercloud"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/nodeinfo"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// Args defines the scheduling parameters for Scheduling plugin.
type Args struct {
	allocator.OpenStackConf
	MaxInstanceIps  int
	MaxInstanceEnis int
}

// Scheduling is a plugin that implements the mechanism of gang scheduling.
type Scheduling struct {
	computeClient   *gophercloud.ServiceClient
	frameworkHandle framework.FrameworkHandle
	// args is scheduling parameters
	args Args
}

var _ framework.QueueSortPlugin = &Scheduling{}
var _ framework.FilterPlugin = &Scheduling{}
var _ framework.PreScorePlugin = &Scheduling{}
var _ framework.ScorePlugin = &Scheduling{}
var _ framework.ScoreExtensions = &Scheduling{}

const (
	// Name is the name of the plugin used in Registry and configurations.
	Name = "Scheduling"
	IpsWeight = 1
	EnisWeigth = 1
)

// Name returns name of the plugin. It is used in logs, etc.
func (cs *Scheduling) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(config *runtime.Unknown, handle framework.FrameworkHandle) (framework.Plugin, error) {
	args := Args{
		MaxInstanceEnis: backend.MAX_ENIS,
		MaxInstanceIps:  backend.MAX_IPS,
	}
	if err := framework.DecodeInto(config, &args); err != nil {
		return nil, err
	}
	computeClient, err := neutron.ConnectStore(&args.OpenStackConf, backend.SERVICE_TYPE_COMPUTE)
	if err != nil {
		return nil, err
	}

	cs := &Scheduling{
		frameworkHandle: handle,
		args:            args,
		computeClient:   computeClient,
	}
	return cs, nil
}

// Less is used to sort pods in the scheduling queue.
// 1. Compare the priorities of Pods.
// 2. Compare the initialization timestamps of PodGroups/Pods.
// 3. Compare the keys of NameSpace/Pods, i.e., if two pods are tied at priority and creation time, the one without namespace will go ahead of the one with namespace.
func (cs *Scheduling) Less(podInfo1, podInfo2 *framework.PodInfo) bool {
	priority1 := podutil.GetPodPriority(podInfo1.Pod)
	priority2 := podutil.GetPodPriority(podInfo2.Pod)

	if priority1 != priority2 {
		return priority1 > priority2
	}

	time1 := podInfo1.InitialAttemptTimestamp
	time2 := podInfo2.InitialAttemptTimestamp

	if !time1.Equal(time2) {
		return time1.Before(time2)
	}

	podKey1 := fmt.Sprintf("%v/%v", podInfo1.Pod.Namespace, podInfo1.Pod.Name)
	podKey2 := fmt.Sprintf("%v/%v", podInfo2.Pod.Namespace, podInfo1.Pod.Name)

	return podKey1 < podKey2
}

func (cs *Scheduling) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *schedulernodeinfo.NodeInfo) *framework.Status {
	nodeInternalIP := cs.getNodeInternalIP(nodeInfo.Node())
	if nodeInternalIP == nil {
		klog.Errorf("Failed to get node %v internal IP", nodeInfo.Node().Name)
		return framework.NewStatus(framework.Unschedulable, "node not have internal IP")
	}

	vmEnis, vmIps, err := neutron.CountServerIntsAndEnis(cs.computeClient, *nodeInternalIP)
	if err != nil {
		klog.Errorf("Failed to get nova instance enis and ips with ip %v, err: %v", *nodeInternalIP, err)
		return framework.NewStatus(framework.Unschedulable, "failed to get instance enis and ips")
	}

	if vmEnis >= cs.args.MaxInstanceEnis {
		klog.V(3).Infof("The enis of instance with ip %v greater than MaxInstanceEnis %v", vmEnis, cs.args.MaxInstanceEnis)
		return framework.NewStatus(framework.Unschedulable, "greater than MaxInstanceEnis")
	}

	if vmIps >= cs.args.MaxInstanceIps {
		klog.V(3).Infof("The ips of instance with ip %v greater than MaxInstanceIps %v", vmIps, cs.args.MaxInstanceIps)
		return framework.NewStatus(framework.Unschedulable, "greater than MaxInstanceIps")
	}

	return framework.NewStatus(framework.Success, "")
}

func (cs *Scheduling) PreScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodes []*v1.Node) *framework.Status {
	return cs.ParallelCollection(nil, 16, state, nodes)
}

func (cs *Scheduling) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	nodeInfo, err := cs.frameworkHandle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Getting node %q from snapshot err: %v", nodeName, err))
	}
	s, err := cs.score(state, nodeInfo)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Score Node err: %v", err))
	}
	klog.V(3).Infof("node %v has score: %v", nodeName, s)
	return s, framework.NewStatus(framework.Success, "")
}

func (cs *Scheduling) NormalizeScore(ctx context.Context, state *framework.CycleState, p *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var highest int64 = 0
	for _, nodeScore := range scores {
		highest = max(highest, nodeScore.Score)
	}
	for i, nodeScore := range scores {
		scores[i].Score = nodeScore.Score * framework.MaxNodeScore / highest
	}
	return framework.NewStatus(framework.Success, "")
}

func (cs *Scheduling) ScoreExtensions() framework.ScoreExtensions {
	return cs
}

func (cs *Scheduling) getNodeInternalIP(node *v1.Node) *string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			return &addr.Address
		}
	}
	return nil
}

func (cs *Scheduling) score(state *framework.CycleState, node *nodeinfo.NodeInfo) (int64, error) {
	ip := cs.getNodeInternalIP(node.Node())
	if ip == nil {
		return 0, fmt.Errorf("node %v internal ip not found", node.Node().Name)
	}

	d, err := state.Read(framework.StateKey(*ip))
	if err != nil {
		klog.V(3).Infof("Failed to get cycleState info err: %v", err)
		return 0, err
	}

	data := d.(*Data)
	score := data.NodeIpsValue * IpsWeight + data.NodeIntsValue * EnisWeigth
	return int64(score), nil
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
