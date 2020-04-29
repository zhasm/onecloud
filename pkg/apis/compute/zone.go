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

package compute

import "yunion.io/x/onecloud/pkg/apis"

type ZoneCreateInput struct {
	apis.StatusStandaloneResourceCreateInput

	// 区域名称或Id,建议使用Id
	Cloudregion string

	// swagger:ignore
	Region string
	// swagger:ignore
	RegionId string
	// swagger:ignore
	CloudregionId string
}

type ZoneDetails struct {
	apis.StandaloneResourceDetails
	SZone

	// 区域名称
	Cloudregion string `json:"cloudregion"`
	// 平台
	// example: OneCloud
	Provider string `json:"provider"`

	// 可用区底下的宿主机数量
	// example: 3
	Hosts int `json:"hosts"`

	// 可用区底下启用的宿主机数量
	// example: 2
	HostsEnabled int `json:"hosts_enabled"`

	// 可用区底下的裸金属服务器数量
	// example: 1
	Baremetals int `json:"baremetals"`

	// 可用区底下启用的裸金属服务器数量
	// example: 1
	BaremetalsEnabled int `json:"baremetals_enabled"`

	// 可用区底下的二层网络数量
	// example: 3
	Wires int `json:"wires"`

	// 可用区底下的子网数量
	// example: 1
	Networks int `json:"networks"`

	// 可用区底下的块存储数量
	// example: 1
	Storages int `json:"storages"`
}

type ZoneInfo struct {
	// 可用区名称
	// example: zone1
	Zone string `json:"zone"`

	// 纳管云的zoneId
	ZoneExtId string `json:"zone_ext_id"`
	CloudregionInfo
}