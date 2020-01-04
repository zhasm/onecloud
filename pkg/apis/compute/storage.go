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

import (
	"yunion.io/x/jsonutils"

	"yunion.io/x/onecloud/pkg/apis"
)

type StorageCreateInput struct {
	apis.StandaloneResourceCreateInput

	// 存储类型
	//
	//
	//
	// | storage_type	| 参数						|是否必传	|	默认值	| 说明		|
	// | --------		| -------					| --------	| --------	| ---------	|
	// | rbd			| rbd_mon_host				| 是		|			| ceph mon_host	|
	// | rbd 			| rbd_pool					| 是 		|			| ceph pool		|
	// | rbd 			| rbd_key					| 否 		|			|若cephx认证开启,此参数必传	|
	// | rbd 			| rbd_rados_mon_op_timeout	| 否 		|	3		|单位: 秒	|
	// | rbd 			| rbd_rados_osd_op_timeout	| 否 		|	1200	|单位: 秒	|
	// | rbd 			| rbd_client_mount_timeout	| 否 		|	120		|单位: 秒	|
	// | nfs 			| nfs_host					| 是 		|			|网络文件系统主机	|
	// | nfs 			| nfs_shared_dir			| 是 		|			|网络文件系统共享目录	|
	// local: 本地存储
	// rbd: ceph块存储, ceph存储创建时仅会检测是否重复创建，不会具体检测认证参数是否合法，只有挂载存储时
	// 计算节点会验证参数，若挂载失败，宿主机和存储不会关联，可以通过查看存储日志查找挂载失败原因
	// enum: local, rbd, nfs, gpfs
	// required: true
	StorageType string `json:"storage_type"`

	// 存储介质类型
	// enum: rotate, ssd, hybird
	// required: true
	MediumType string `json:"medium_type"`

	// 可用区名称或ID, 建议使用ID
	// required: true
	Zone string `json:"zone"`

	// swagger:ignore
	ZoneId string

	// ceph认证主机, storage_type为 rbd 时,此参数为必传项
	// 单个ip或以逗号分隔的多个ip具体可查询 /etc/ceph/ceph.conf 文件
	// example: 192.168.222.3,192.168.222.4,192.168.222.99
	RbdMonHost string `json:"rbd_mon_host"`

	// swagger:ignore
	MonHost string

	// ceph使用的pool, storage_type为 rbd 时,此参数为必传项
	// example: rbd
	RbdPool string `json:"rbd_pool"`

	// swagger:ignore
	Pool string

	// ceph集群密码,若ceph集群开启cephx认证,此参数必传
	// 可在ceph集群主机的/etc/ceph/ceph.client.admin.keyring文件中找到
	// example: AQDigB9dtnDAKhAAxS6X4zi4BPR/lIle4nf4Dw==
	RbdKey string `json:"rbd_key"`

	// swagger:ignore
	Key string

	// ceph集群连接超时时间, 单位秒
	// default: 3
	RbdRadosMonOpTimeout int `json:"rbd_rados_mon_op_timeout"`

	// swagger:ignore
	RadosMonOpTimeout int

	// ceph osd 操作超时时间, 单位秒
	// default: 1200
	RbdRadosOsdOpTimeout int `json:"rbd_rados_osd_op_timeout"`

	// swagger:ignore
	RadosOsdOpTimeout int

	// ceph CephFS挂载超时时间, 单位秒
	// default: 120
	RbdClientMountTimeout int `json:"rbd_client_mount_timeout"`

	// swagger:ignore
	ClientMountTimeout int

	// swagger:ignore
	StorageConf *jsonutils.JSONDict

	// 网络文件系统主机, storage_type 为 nfs 时,此参数必传
	// example: 192.168.222.2
	NfsHost string `json:"nfs_host"`

	// 网络文件系统共享目录, storage_type 为 nfs 时, 此参数必传
	// example: /nfs_root/
	NfsSharedDir string `json:"nfs_shared_dir"`
}
