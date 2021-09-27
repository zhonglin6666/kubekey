package kubernetes

import (
	"github.com/kubesphere/kubekey/pkg/core/action"
	"github.com/kubesphere/kubekey/pkg/core/modules"
	"github.com/kubesphere/kubekey/pkg/core/prepare"
	"github.com/kubesphere/kubekey/pkg/pipelines/common"
	"github.com/kubesphere/kubekey/pkg/pipelines/kubernetes/templates"
	"path/filepath"
)

type KubernetesStatusModule struct {
	common.KubeModule
}

func (k *KubernetesStatusModule) Init() {
	k.Name = "KubernetesStatusModule"

	cluster := NewKubernetesStatus()
	k.PipelineCache.GetOrSet(common.ClusterStatus, cluster)

	clusterStatus := &modules.Task{
		Name:  "GetClusterStatus",
		Desc:  "get kubernetes cluster status",
		Hosts: k.Runtime.GetHostsByRole(common.Master),
		//Prepare:  new(NoClusterInfo),
		Action:   new(GetClusterStatus),
		Parallel: false,
	}

	k.Tasks = []*modules.Task{
		clusterStatus,
	}
}

type InstallKubeBinariesModule struct {
	common.KubeModule
}

func (i *InstallKubeBinariesModule) Init() {
	i.Name = "InstallKubeBinariesModule"

	syncBinary := &modules.Task{
		Name:     "SyncKubeBinary",
		Desc:     "synchronize kubernetes binaries",
		Hosts:    i.Runtime.GetHostsByRole(common.K8s),
		Prepare:  &NodeInCluster{Not: true},
		Action:   new(SyncKubeBinary),
		Parallel: true,
		Retry:    2,
	}

	syncKubelet := &modules.Task{
		Name:     "SyncKubelet",
		Desc:     "synchronize kubelet",
		Hosts:    i.Runtime.GetHostsByRole(common.K8s),
		Prepare:  &NodeInCluster{Not: true},
		Action:   new(SyncKubelet),
		Parallel: true,
		Retry:    2,
	}

	generateKubeletService := &modules.Task{
		Name:    "GenerateKubeletService",
		Desc:    "generate kubelet service",
		Hosts:   i.Runtime.GetHostsByRole(common.K8s),
		Prepare: &NodeInCluster{Not: true},
		Action: &action.Template{
			Template: templates.KubeletService,
			Dst:      filepath.Join("/etc/systemd/system/", templates.KubeletService.Name()),
		},
		Parallel: true,
		Retry:    2,
	}

	enableKubelet := &modules.Task{
		Name:     "EnableKubelet",
		Desc:     "enable kubelet service",
		Hosts:    i.Runtime.GetHostsByRole(common.K8s),
		Prepare:  &NodeInCluster{Not: true},
		Action:   new(EnableKubelet),
		Parallel: true,
		Retry:    5,
	}

	generateKubeletEnv := &modules.Task{
		Name:     "GenerateKubeletEnv",
		Desc:     "generate kubelet env",
		Hosts:    i.Runtime.GetHostsByRole(common.K8s),
		Prepare:  &NodeInCluster{Not: true},
		Action:   new(GenerateKubeletEnv),
		Parallel: true,
		Retry:    2,
	}

	i.Tasks = []*modules.Task{
		syncBinary,
		syncKubelet,
		generateKubeletService,
		enableKubelet,
		generateKubeletEnv,
	}
}

type InitKubernetesModule struct {
	common.KubeModule
}

func (i *InitKubernetesModule) Init() {
	i.Name = "InitKubernetesModule"

	generateKubeadmConfig := &modules.Task{
		Name:  "GenerateKubeadmConfig",
		Desc:  "generate kubeadm config",
		Hosts: i.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			new(common.OnlyFirstMaster),
			&ClusterIsExist{Not: true},
		},
		Action:   &GenerateKubeadmConfig{IsInitConfiguration: true},
		Parallel: true,
	}

	kubeadmInit := &modules.Task{
		Name:  "KubeadmInit",
		Desc:  "init cluster using kubeadm",
		Hosts: i.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			new(common.OnlyFirstMaster),
			&ClusterIsExist{Not: true},
		},
		Action:   new(KubeadmInit),
		Retry:    3,
		Parallel: true,
	}

	copyKubeConfig := &modules.Task{
		Name:  "CopyKubeConfig",
		Desc:  "copy admin.conf to ~/.kube/config",
		Hosts: i.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			new(common.OnlyFirstMaster),
			&ClusterIsExist{Not: true},
		},
		Action:   new(CopyKubeConfigForControlPlane),
		Parallel: true,
	}

	removeMasterTaint := &modules.Task{
		Name:  "RemoveMasterTaint",
		Desc:  "remove master taint",
		Hosts: i.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			new(common.OnlyFirstMaster),
			&ClusterIsExist{Not: true},
			new(common.IsWorker),
		},
		Action:   new(RemoveMasterTaint),
		Parallel: true,
		Retry:    5,
	}

	addWorkerLabel := &modules.Task{
		Name:  "AddWorkerLabel",
		Desc:  "add worker label",
		Hosts: i.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			new(common.OnlyFirstMaster),
			&ClusterIsExist{Not: true},
			new(common.IsWorker),
		},
		Action:   new(AddWorkerLabel),
		Parallel: true,
		Retry:    5,
	}

	i.Tasks = []*modules.Task{
		generateKubeadmConfig,
		kubeadmInit,
		copyKubeConfig,
		removeMasterTaint,
		addWorkerLabel,
	}
}

type JoinNodesModule struct {
	common.KubeModule
}

func (j *JoinNodesModule) Init() {
	j.Name = "JoinNodesModule"

	j.PipelineCache.Set(common.ClusterExist, true)

	generateKubeadmConfig := &modules.Task{
		Name:  "GenerateKubeadmConfig",
		Desc:  "generate kubeadm config",
		Hosts: j.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
		},
		Action:   &GenerateKubeadmConfig{IsInitConfiguration: false},
		Parallel: true,
	}

	joinNode := &modules.Task{
		Name:  "JoinNode",
		Desc:  "Join node",
		Hosts: j.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
		},
		Action:   new(JoinNode),
		Parallel: true,
		Retry:    5,
	}

	copyKubeConfig := &modules.Task{
		Name:  "copyKubeConfig",
		Desc:  "copy admin.conf to ~/.kube/config",
		Hosts: j.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
		},
		Action:   new(CopyKubeConfigForControlPlane),
		Parallel: true,
		Retry:    2,
	}

	removeMasterTaint := &modules.Task{
		Name:  "RemoveMasterTaint",
		Desc:  "remove master taint",
		Hosts: j.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
			new(common.IsWorker),
		},
		Action:   new(RemoveMasterTaint),
		Parallel: true,
		Retry:    5,
	}

	addWorkerLabelToMaster := &modules.Task{
		Name:  "AddWorkerLabelToMaster",
		Desc:  "add worker label to master",
		Hosts: j.Runtime.GetHostsByRole(common.Master),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
			new(common.IsWorker),
		},
		Action:   new(AddWorkerLabel),
		Parallel: true,
		Retry:    5,
	}

	syncKubeConfig := &modules.Task{
		Name:  "SyncKubeConfig",
		Desc:  "synchronize kube config to worker",
		Hosts: j.Runtime.GetHostsByRole(common.Worker),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
			new(common.OnlyWorker),
		},
		Action:   new(SyncKubeConfigToWorker),
		Parallel: true,
		Retry:    3,
	}

	addWorkerLabelToWorker := &modules.Task{
		Name:  "AddWorkerLabelToWorker",
		Desc:  "add worker label to worker",
		Hosts: j.Runtime.GetHostsByRole(common.Worker),
		Prepare: &prepare.PrepareCollection{
			&NodeInCluster{Not: true},
			new(common.OnlyWorker),
		},
		Action:   new(AddWorkerLabel),
		Parallel: true,
		Retry:    5,
	}

	j.Tasks = []*modules.Task{
		generateKubeadmConfig,
		joinNode,
		copyKubeConfig,
		removeMasterTaint,
		addWorkerLabelToMaster,
		syncKubeConfig,
		addWorkerLabelToWorker,
	}
}

type ResetClusterModule struct {
	common.KubeModule
}

func (r *ResetClusterModule) Init() {
	r.Name = "ResetClusterModule"

	kubeadmReset := &modules.Task{
		Name:     "KubeadmReset",
		Desc:     "Reset the cluster using kubeadm",
		Hosts:    r.Runtime.GetHostsByRole(common.K8s),
		Action:   new(KubeadmReset),
		Parallel: true,
	}

	r.Tasks = []*modules.Task{
		kubeadmReset,
	}
}

type CompareConfigAndClusterInfoModule struct {
	common.KubeModule
}

func (c *CompareConfigAndClusterInfoModule) Init() {
	c.Name = "CompareConfigAndClusterInfoModule"

	check := &modules.Task{
		Name:    "FindDifferences",
		Desc:    "Find the differences between config and cluster node info",
		Hosts:   c.Runtime.GetHostsByRole(common.Master),
		Prepare: new(common.OnlyFirstMaster),
		Action:  new(FindDifferences),
	}

	c.Tasks = []*modules.Task{
		check,
	}
}

type DeleteKubeNodeModule struct {
	common.KubeModule
}

func (r *DeleteKubeNodeModule) Init() {
	r.Name = "DeleteKubeNodeModule"

	drain := &modules.Task{
		Name:    "DrainNode",
		Desc:    "Node safely evict all pods",
		Hosts:   r.Runtime.GetHostsByRole(common.Master),
		Prepare: new(common.OnlyFirstMaster),
		Action:  new(DrainNode),
		Retry:   5,
	}

	deleteNode := &modules.Task{
		Name:    "DeleteNode",
		Desc:    "Delete the node using kubectl",
		Hosts:   r.Runtime.GetHostsByRole(common.Master),
		Prepare: new(common.OnlyFirstMaster),
		Action:  new(KubectlDeleteNode),
		Retry:   5,
	}

	r.Tasks = []*modules.Task{
		drain,
		deleteNode,
	}
}