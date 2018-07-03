/*
Copyright 2016 The Kubernetes Authors.

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

package qcloud

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/config/dynamic"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

const (
	// ProviderName is the cloud provider name for QCLOUD
	ProviderName = "qcloud"
)

// qcloudCloudProvider implements CloudProvider interface.
type qcloudCloudProvider struct {
	qcloudManager *QcloudManager
	asgs       []*Asg
	resourceLimiter *cloudprovider.ResourceLimiter
}

// BuildQcloudCloudProvider builds CloudProvider implementation for QCLOUD.
func BuildQcloudCloudProvider(qcloudManager *QcloudManager, discoveryOpts cloudprovider.NodeGroupDiscoveryOptions, resourceLimiter *cloudprovider.ResourceLimiter) (cloudprovider.CloudProvider, error) {
	if err := discoveryOpts.Validate(); err != nil {
		return nil, fmt.Errorf("Failed to build an qcloud cloud provider: %v", err)
	}
	if discoveryOpts.StaticDiscoverySpecified() {
		return buildStaticallyDiscoveringProvider(qcloudManager, discoveryOpts.NodeGroupSpecs, resourceLimiter)
	}

	return nil, fmt.Errorf("Failed to build an qcloud cloud provider: Either node group specs or node group auto discovery spec must be specified")
}


func buildStaticallyDiscoveringProvider(qcloudManager *QcloudManager, specs []string, resourceLimiter *cloudprovider.ResourceLimiter) (*qcloudCloudProvider, error) {
	qcloud := &qcloudCloudProvider{
		qcloudManager: qcloudManager,
		asgs:       make([]*Asg, 0),
		resourceLimiter:resourceLimiter,
	}
	for _, spec := range specs {
		if err := qcloud.addNodeGroup(spec); err != nil {
			return nil, err
		}
	}
	return qcloud, nil
}

// addNodeGroup adds node group defined in string spec. Format:
// minNodes:maxNodes:asgName
func (qcloud *qcloudCloudProvider) addNodeGroup(spec string) error {
	asg, err := buildAsgFromSpec(spec, qcloud.qcloudManager)
	if err != nil {
		return err
	}
	qcloud.addAsg(asg)
	return nil
}

func (qcloud *qcloudCloudProvider) Cleanup() error {
	qcloud.qcloudManager.Cleanup()
	return nil
}

// addAsg adds and registers an asg to this cloud provider
func (qcloud *qcloudCloudProvider) addAsg(asg *Asg) {
	qcloud.asgs = append(qcloud.asgs, asg)
	qcloud.qcloudManager.RegisterAsg(asg)
}

// Name returns name of the cloud provider.
func (qcloud *qcloudCloudProvider) Name() string {
	return "qcloud"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (qcloud *qcloudCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(qcloud.asgs))
	for _, asg := range qcloud.asgs {
		result = append(result, asg)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (qcloud *qcloudCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	ref, err := QcloudRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	asg, err := qcloud.qcloudManager.GetAsgForInstance(ref)
	return asg, err
}

// Pricing returns pricing model for this cloud provider or error if not available.
func (qcloud *qcloudCloudProvider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetAvailableMachineTypes get all machine types that can be requested from the cloud provider.
func (qcloud *qcloudCloudProvider) GetAvailableMachineTypes() ([]string, error) {
	return []string{}, nil
}

// NewNodeGroup builds a theoretical node group based on the node definition provided. The node group is not automatically
// created on the cloud provider side. The node group is not returned by NodeGroups() until it is created.
func (qcloud *qcloudCloudProvider) NewNodeGroup(machineType string, labels map[string]string, extraResources map[string]resource.Quantity) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// GetResourceLimiter returns struct containing limits (max, min) for resources (cores, memory etc.).
func (qcloud *qcloudCloudProvider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return qcloud.resourceLimiter, nil
}

// Refresh is called before every main loop and can be used to dynamically update cloud provider state.
// In particular the list of node groups returned by NodeGroups can change as a result of CloudProvider.Refresh().
func (qcloud *qcloudCloudProvider) Refresh() error {
	return nil
}

// QcloudRef contains a reference to some entity in QCLOUD/GKE world.
type QcloudRef struct {
	Name string
}

// QcloudRefFromProviderId creates InstanceConfig object from provider id which
// must be in format: qcloud:///100003/ins-3ven36lk
func QcloudRefFromProviderId(id string) (*QcloudRef, error) {
	validIdRegex := regexp.MustCompile(`^qcloud\:\/\/\/[-0-9a-z]*\/[-0-9a-z]*$`)
	if validIdRegex.FindStringSubmatch(id) == nil {
		return nil, fmt.Errorf("Wrong id: expected format qcloud:///zoneid/ins-<name>, got %v", id)
	}
	splitted := strings.Split(id[10:], "/")
	return &QcloudRef{
		Name: splitted[1],
	}, nil
}

// Asg implements NodeGroup interface.
type Asg struct {
	QcloudRef

	qcloudManager *QcloudManager

	minSize int
	maxSize int
}

// MaxSize returns maximum size of the node group.
func (asg *Asg) MaxSize() int {
	return asg.maxSize
}

// MinSize returns minimum size of the node group.
func (asg *Asg) MinSize() int {
	return asg.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kuberentes.
func (asg *Asg) TargetSize() (int, error) {
	size, err := asg.qcloudManager.GetAsgSize(asg)
	return int(size), err
}

// IncreaseSize increases Asg size
func (asg *Asg) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := asg.qcloudManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size)+delta > asg.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, asg.MaxSize())
	}
	return asg.qcloudManager.SetAsgSize(asg, size+int64(delta))
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes if the size
// when there is an option to just decrease the target.
func (asg *Asg) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease size must be negative")
	}
	size, err := asg.qcloudManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	nodes, err := asg.qcloudManager.GetAsgNodes(asg)
	if err != nil {
		return err
	}
	if int(size)+delta < len(nodes) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%d delta:%d existingNodes: %d",
			size, delta, len(nodes))
	}
	return asg.qcloudManager.SetAsgSize(asg, size+int64(delta))
}

// Belongs returns true if the given node belongs to the NodeGroup.
func (asg *Asg) Belongs(node *apiv1.Node) (bool, error) {
	ref, err := QcloudRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetAsg, err := asg.qcloudManager.GetAsgForInstance(ref)
	if err != nil {
		return false, err
	}
	if targetAsg == nil {
		return false, fmt.Errorf("%s doesn't belong to a known asg", node.Name)
	}
	if targetAsg.Id() != asg.Id() {
		return false, nil
	}
	return true, nil
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one.
func (asg *Asg) Exist() bool {
	return true
}

// Create creates the node group on the cloud provider side.
func (asg *Asg) Create() error {
	return cloudprovider.ErrAlreadyExist
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
func (asg *Asg) Delete() error {
	return cloudprovider.ErrNotImplemented
}

// Autoprovisioned returns true if the node group is autoprovisioned.
func (asg *Asg) Autoprovisioned() bool {
	return false
}

// DeleteNodes deletes the nodes from the group.
func (asg *Asg) DeleteNodes(nodes []*apiv1.Node) error {
	size, err := asg.qcloudManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size) <= asg.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	refs := make([]*QcloudRef, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := asg.Belongs(node)
		if err != nil {
			return err
		}
		if belongs != true {
			return fmt.Errorf("%s,%s belongs to a different asg than %s", node.Name, node.Spec.ProviderID, asg.Id())
		}
		qcloudref, err := QcloudRefFromProviderId(node.Spec.ProviderID)
		if err != nil {
			return err
		}
		refs = append(refs, qcloudref)
	}
	return asg.qcloudManager.DeleteInstances(refs)
}

// Id returns asg id.
func (asg *Asg) Id() string {
	return asg.Name
}

// Debug returns a debug string for the Asg.
func (asg *Asg) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", asg.Id(), asg.MinSize(), asg.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
func (asg *Asg) Nodes() ([]string, error) {
	return asg.qcloudManager.GetAsgNodes(asg)
}

func (asg *Asg) TemplateNodeInfo() (*schedulercache.NodeInfo, error) {
	template, err := asg.qcloudManager.getAsgTemplate(asg.Name)
	if err != nil {
		return nil, err
	}

	node, err := asg.qcloudManager.buildNodeFromTemplate(asg, template)
	if err != nil {
		return nil, err
	}

	nodeInfo := schedulercache.NewNodeInfo()
	nodeInfo.SetNode(node)
	return nodeInfo, nil
}


func buildAsgFromSpec(value string, qcloudManager *QcloudManager) (*Asg, error) {
	spec, err := dynamic.SpecFromString(value, true)

	if err != nil {
		return nil, fmt.Errorf("failed to parse node group spec: %v", err)
	}

	asg := buildAsg(qcloudManager, spec.MinSize, spec.MaxSize, spec.Name)

	return asg, nil
}

func buildAsg(qcloudManager *QcloudManager, minSize int, maxSize int, name string) *Asg {
	return &Asg{
		qcloudManager: qcloudManager,
		minSize:    minSize,
		maxSize:    maxSize,
		QcloudRef: QcloudRef{
			Name: name,
		},
	}
}
