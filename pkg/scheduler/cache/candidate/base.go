// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package candidate

import (
	"fmt"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/scheduler/api"

	computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	computedb "yunion.io/x/onecloud/pkg/cloudcommon/db"
	computemodels "yunion.io/x/onecloud/pkg/compute/models"
	schedmodels "yunion.io/x/onecloud/pkg/scheduler/models"
)

type BaseHostDesc struct {
	*computemodels.SHost
	Region        *computemodels.SCloudregion              `json:"region"`
	Zone          *computemodels.SZone                     `json:"zone"`
	Cloudprovider *computemodels.SCloudprovider            `json:"cloudprovider"`
	Networks      []*api.CandidateNetwork                  `json:"networks"`
	NetInterfaces map[string][]computemodels.SNetInterface `json:"net_interfaces"`
	Storages      []*api.CandidateStorage                  `json:"storages"`

	Tenants       map[string]int64          `json:"tenants"`
	HostSchedtags []computemodels.SSchedtag `json:"schedtags"`
}

type baseHostGetter struct {
	h *BaseHostDesc
}

func newBaseHostGetter(h *BaseHostDesc) *baseHostGetter {
	return &baseHostGetter{h}
}

func (b baseHostGetter) Id() string {
	return b.h.GetId()
}

func (b baseHostGetter) Name() string {
	return b.h.GetName()
}

func (b baseHostGetter) Zone() *computemodels.SZone {
	return b.h.Zone
}

func (b baseHostGetter) Host() *computemodels.SHost {
	return b.h.SHost
}

func (b baseHostGetter) Cloudprovider() *computemodels.SCloudprovider {
	return b.h.Cloudprovider
}

func (b baseHostGetter) IsPublic() bool {
	provider := b.Cloudprovider()
	if provider == nil {
		return false
	}
	account := provider.GetCloudaccount()
	if account == nil {
		return false
	}
	return account.GetIsPublic()
}

func (b baseHostGetter) DomainId() string {
	provider := b.Cloudprovider()
	if provider == nil {
		return ""
	}
	return provider.DomainId
}

func (b baseHostGetter) Region() *computemodels.SCloudregion {
	return b.h.Region
}

func (b baseHostGetter) HostType() string {
	return b.h.HostType
}

func (b baseHostGetter) HostSchedtags() []computemodels.SSchedtag {
	return b.h.HostSchedtags
}

func (b baseHostGetter) Storages() []*api.CandidateStorage {
	return b.h.Storages
}

func (b baseHostGetter) Networks() []*api.CandidateNetwork {
	return b.h.Networks
}

func (b baseHostGetter) ResourceType() string {
	return reviseResourceType(b.h.ResourceType)
}

func (b baseHostGetter) NetInterfaces() map[string][]computemodels.SNetInterface {
	return b.h.NetInterfaces
}

func (b baseHostGetter) Status() string {
	return b.h.Status
}

func (b baseHostGetter) HostStatus() string {
	return b.h.HostStatus
}

func (b baseHostGetter) Enabled() bool {
	return b.h.Enabled
}

func (b baseHostGetter) ProjectGuests() map[string]int64 {
	return b.h.Tenants
}

func (b baseHostGetter) CreatingGuestCount() int {
	return 0
}

func (b baseHostGetter) RunningCPUCount() int64 {
	return 0
}

func (b baseHostGetter) TotalCPUCount(_ bool) int64 {
	return int64(b.h.CpuCount)
}

func (b baseHostGetter) RunningMemorySize() int64 {
	return 0
}

func (b baseHostGetter) TotalMemorySize(_ bool) int64 {
	return int64(b.h.MemSize)
}

func (b baseHostGetter) GetFreeStorageSizeOfType(storageType string, useRsvd bool) int64 {
	var size int64
	for _, s := range b.Storages() {
		if s.StorageType == storageType {
			size += int64(float32(s.Capacity) * s.Cmtbound)
		}
	}
	return size
}

func (b baseHostGetter) GetFreePort(netId string) int {
	return b.h.GetFreePort(netId)
}

func reviseResourceType(resType string) string {
	if resType == "" {
		return computeapi.HostResourceTypeDefault
	}
	return resType
}

func newBaseHostDesc(host *computemodels.SHost) (*BaseHostDesc, error) {
	host.ResourceType = reviseResourceType(host.ResourceType)
	desc := &BaseHostDesc{
		SHost: host,
	}

	if err := desc.fillCloudProvider(host); err != nil {
		return nil, fmt.Errorf("Fill cloudprovider info error: %v", err)
	}

	if err := desc.fillNetworks(host); err != nil {
		return nil, fmt.Errorf("Fill networks error: %v", err)
	}

	if err := desc.fillZone(host); err != nil {
		return nil, fmt.Errorf("Fill zone error: %v", err)
	}

	if err := desc.fillRegion(host); err != nil {
		return nil, fmt.Errorf("Fill region error: %v", err)
	}

	if err := desc.fillResidentTenants(host); err != nil {
		return nil, fmt.Errorf("Fill resident tenants error: %v", err)
	}

	if err := desc.fillStorages(host); err != nil {
		return nil, fmt.Errorf("Fill storage error: %v", err)
	}

	if err := desc.fillSchedtags(); err != nil {
		return nil, fmt.Errorf("Fill schedtag error: %v", err)
	}

	return desc, nil
}

func (b BaseHostDesc) GetSchedDesc() *jsonutils.JSONDict {
	desc := jsonutils.Marshal(b.SHost).(*jsonutils.JSONDict)

	if b.Cloudprovider != nil {
		p := b.Cloudprovider
		cloudproviderDesc := jsonutils.NewDict()
		cloudproviderDesc.Add(jsonutils.NewString(p.ProjectId), "tenant_id")
		cloudproviderDesc.Add(jsonutils.NewString(p.Provider), "provider")
		desc.Add(cloudproviderDesc, "cloudprovider")
	}

	return desc
}

func (b *BaseHostDesc) GetPendingUsage() *schedmodels.SPendingUsage {
	usage, err := schedmodels.HostPendingUsageManager.GetPendingUsage(b.GetId())
	if err != nil {
		return schedmodels.NewPendingUsageBySchedInfo(b.GetId(), nil)
	}
	return usage
}

func (b *BaseHostDesc) GetFreePort(netId string) int {
	var selNet *api.CandidateNetwork = nil
	for _, n := range b.Networks {
		if n.GetId() == netId {
			selNet = n
			break
		}
	}
	if selNet == nil {
		return 0
	}
	freeCount, _ := selNet.GetFreeAddressCount()
	return freeCount
}

func (b BaseHostDesc) GetResourceType() string {
	return b.ResourceType
}

func (b *BaseHostDesc) fillCloudProvider(host *computemodels.SHost) error {
	b.Cloudprovider = host.GetCloudprovider()
	return nil
}

func (b *BaseHostDesc) fillRegion(host *computemodels.SHost) error {
	b.Region = host.GetRegion()
	return nil
}

func (b *BaseHostDesc) fillZone(host *computemodels.SHost) error {
	zone := host.GetZone()
	b.Zone = zone
	b.ZoneId = host.ZoneId
	return nil
}

func (b *BaseHostDesc) fillResidentTenants(host *computemodels.SHost) error {
	rets, err := HostResidentTenantCount(host.Id)
	if err != nil {
		return err
	}

	b.Tenants = rets

	return nil
}

func (b *BaseHostDesc) fillSchedtags() error {
	b.HostSchedtags = b.SHost.GetSchedtags()
	return nil
}

func (b *BaseHostDesc) fillNetworks(host *computemodels.SHost) error {
	hostId := host.Id
	hostwires := computemodels.HostwireManager.Query().SubQuery()
	sq := hostwires.Query(sqlchemy.DISTINCT("wire_id", hostwires.Field("wire_id"))).Equals("host_id", hostId)
	networks := computemodels.NetworkManager.Query().SubQuery()
	q := networks.Query().In("wire_id", sq)

	nets := make([]computemodels.SNetwork, 0)
	err := computedb.FetchModelObjects(computemodels.NetworkManager, q, &nets)
	if err != nil {
		return err
	}
	b.Networks = make([]*api.CandidateNetwork, len(nets))
	for idx, n := range nets {
		b.Networks[idx] = &api.CandidateNetwork{
			SNetwork:  &nets[idx],
			Schedtags: n.GetSchedtags(),
		}
	}

	netifs := host.GetNetInterfaces()
	netifIndexs := make(map[string][]computemodels.SNetInterface, 0)
	for _, netif := range netifs {
		if !netif.IsUsableServernic() {
			continue
		}
		wire := netif.GetWire()
		if wire == nil {
			continue
		}
		if _, exist := netifIndexs[wire.Id]; !exist {
			netifIndexs[wire.Id] = make([]computemodels.SNetInterface, 0)
		}
		netifIndexs[wire.Id] = append(netifIndexs[wire.Id], netif)
	}
	b.NetInterfaces = netifIndexs

	return nil
}

func (b *BaseHostDesc) fillStorages(host *computemodels.SHost) error {
	ss := make([]*api.CandidateStorage, 0)
	for _, s := range host.GetHoststorages() {
		storage := s.GetStorage()
		ss = append(ss, &api.CandidateStorage{
			SStorage:  storage,
			Schedtags: storage.GetSchedtags(),
		})
	}
	b.Storages = ss
	return nil
}

func (h *BaseHostDesc) GetEnableStatus() string {
	if h.Enabled {
		return "enable"
	}
	return "disable"
}

func (h *BaseHostDesc) GetHostType() string {
	if h.HostType == api.HostTypeBaremetal && h.IsBaremetal {
		return api.HostTypeBaremetal
	}
	return h.HostType
}

func HostsResidentTenantStats(hostIDs []string) (map[string]map[string]interface{}, error) {
	residentTenantStats, err := FetchHostsResidentTenants(hostIDs)
	if err != nil {
		return nil, err
	}
	stat3 := make([]utils.StatItem3, len(residentTenantStats))
	for i, item := range residentTenantStats {
		stat3[i] = item
	}
	return utils.ToStatDict3(stat3)
}

func HostResidentTenantCount(id string) (map[string]int64, error) {
	residentTenantDict, err := HostsResidentTenantStats([]string{id})
	if err != nil {
		return nil, err
	}
	tenantMap, ok := residentTenantDict[id]
	if !ok {
		log.V(10).Infof("Not found host ID: %s when fill resident tenants, may be no guests on it.", id)
		return nil, nil
	}
	rets := make(map[string]int64, len(tenantMap))
	for tenantID, countObj := range tenantMap {
		rets[tenantID] = countObj.(int64)
	}
	return rets, nil
}

type DescBuilder struct {
	actor BuildActor
}

func NewDescBuilder(act BuildActor) *DescBuilder {
	return &DescBuilder{
		actor: act,
	}
}

func (d *DescBuilder) Build(ids []string) ([]interface{}, error) {
	return d.actor.Do(ids)
}
