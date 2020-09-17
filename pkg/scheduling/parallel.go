package scheduling

import (
	"context"
	"github.com/yaoice/meliodas/pkg/ipam/backend"
	"github.com/yaoice/meliodas/pkg/ipam/backend/neutron"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"sync"
)


type Data struct {
	NodeIntsValue int
	NodeIpsValue int
}

func (s *Data) Clone() framework.StateData {
	c := &Data{
		NodeIntsValue: s.NodeIntsValue,
		NodeIpsValue: s.NodeIpsValue,
	}
	return c
}

func (cs *Scheduling) CollectNodeIntsAndIps(node *v1.Node, state *framework.CycleState) *framework.Status {
	nodeInternalIP := cs.getNodeInternalIP(node)
	if nodeInternalIP == nil {
		return framework.NewStatus(framework.Error, "no internal ip")
	}
	vmEnis, vmIps, err := neutron.CountServerIntsAndEnis(cs.computeClient, *nodeInternalIP)
	if err != nil {
			klog.Errorf("Failed to get nova instance enis and ips with ip %v, err: %v", *nodeInternalIP, err.Error())
			return framework.NewStatus(framework.Error, "failed to get instance enis and ips")
	}
	state.Lock()
	state.Write(framework.StateKey(*nodeInternalIP), &Data{
		NodeIntsValue: backend.MAX_IPS - vmEnis,
		NodeIpsValue:  backend.MAX_ENIS - vmIps,
	})
	defer state.Unlock()
	return framework.NewStatus(framework.Success, "")
}


func (cs *Scheduling) ParallelCollection(ctx context.Context, workers int, state *framework.CycleState, nodes []*v1.Node) *framework.Status {
	var (
		stop <-chan struct{}
		mx sync.RWMutex
		msg string
	)
	if ctx != nil {
		stop = ctx.Done()
	}

	pieces := len(nodes)
	toProcess := make(chan *v1.Node, pieces)
	for i := 0; i < pieces; i++ {
		toProcess <- nodes[i]
	}
	close(toProcess)

	if pieces < workers {
		workers = pieces
	}

	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			for node := range toProcess {
				select {
				case <-stop:
					return
				default:
					if re := cs.CollectNodeIntsAndIps(node, state); !re.IsSuccess() {
						klog.V(3).Infof(re.Message())
						mx.Lock()
						msg += re.Message()
						mx.Unlock()
					}
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if msg != "" {
		return framework.NewStatus(framework.Error, msg)
	}
	return framework.NewStatus(framework.Success, "")
}
