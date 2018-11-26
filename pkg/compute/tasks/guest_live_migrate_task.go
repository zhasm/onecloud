package tasks

import (
	"context"
	"fmt"
	"net/http"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/utils"

	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/notifyclient"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

type GuestMigrateTask struct {
	SSchedTask
}

type GuestLiveMigrateTask struct {
	GuestMigrateTask
}

func init() {
	taskman.RegisterTask(GuestLiveMigrateTask{})
	taskman.RegisterTask(GuestMigrateTask{})
}

func (self *GuestMigrateTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	StartScheduleObjects(ctx, self, []db.IStandaloneModel{obj})
}

func (self *GuestMigrateTask) GetSchedParams() *jsonutils.JSONDict {
	obj := self.GetObject()
	guest := obj.(*models.SGuest)
	schedDesc := guest.ToSchedDesc()
	if self.Params.Contains("prefer_host_id") {
		preferHostId, _ := self.Params.Get("prefer_host_id")
		schedDesc.Set("prefer_host_id", preferHostId)
	}
	return schedDesc
}

func (self *GuestMigrateTask) OnStartSchedule(obj IScheduleModel) {
	guest := obj.(*models.SGuest)
	guest.SetStatus(self.UserCred, models.VM_MIGRATING, "")
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATING, "", self.UserCred)
}

func (self *GuestMigrateTask) OnScheduleFailCallback(obj IScheduleModel, reason string) {
	// do nothing
}

func (self *GuestMigrateTask) OnScheduleFailed(ctx context.Context, reason string) {
	obj := self.GetObject()
	guest := obj.(*models.SGuest)
	self.TaskFailed(ctx, guest, reason)
}

func (self *GuestMigrateTask) SaveScheduleResult(ctx context.Context, obj IScheduleModel, targetHostId string) {
	guest := obj.(*models.SGuest)
	targetHost := models.HostManager.FetchHostById(targetHostId)
	if targetHost == nil {
		self.TaskFailed(ctx, guest, "target host not found?")
		return
	}
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATING, fmt.Sprintf("guest start migrate from host %s to %s", guest.HostId, targetHostId), self.UserCred)

	body := jsonutils.NewDict()
	body.Set("target_host_id", jsonutils.NewString(targetHostId))

	disks := guest.GetDisks()
	disk := disks[0].GetDisk()
	isLocalStorage := utils.IsInStringArray(disk.GetStorage().StorageType,
		models.STORAGE_LOCAL_TYPES)
	if isLocalStorage {
		body.Set("is_local_storage", jsonutils.JSONTrue)
	} else {
		body.Set("is_local_storage", jsonutils.JSONFalse)
	}

	self.SetStage("OnCachedImageComplete", body)
	// prepare disk for migration
	if isLocalStorage {
		targetStorageCache := targetHost.GetLocalStoragecache()
		if targetStorageCache != nil {
			targetStorageCache.StartImageCacheTask(ctx, self.UserCred, disk.TemplateId, false, self.GetTaskId())
		}
	} else {
		self.OnSrcPrepareComplete(ctx, guest, nil)
	}
}

// For local storage get disk info
func (self *GuestMigrateTask) OnCachedImageComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	header := http.Header{}
	header.Set("X-Auth-Token", self.GetUserCred().GetTokenString())
	header.Set("X-Task-Id", self.GetTaskId())
	header.Set("X-Region-Version", "v2")
	body := jsonutils.NewDict()
	guestStatus, _ := self.Params.GetString("guest_status")
	if !jsonutils.QueryBoolean(self.Params, "is_rescue_mode", false) && (guestStatus == models.VM_RUNNING || guestStatus == models.VM_SUSPEND) {
		body.Set("live_migrate", jsonutils.JSONTrue)
	}

	host := guest.GetHost()
	url := fmt.Sprintf("%s/servers/%s/src-prepare-migrate", host.ManagerUri, guest.Id)
	self.SetStage("OnSrcPrepareComplete", nil)
	_, _, err := httputils.JSONRequest(httputils.GetDefaultClient(), ctx, "POST",
		url, header, body, false)
	if err != nil {
		self.TaskFailed(ctx, guest, fmt.Sprintf("Prepare migrage failed: %s", err))
		return
	}
}

func (self *GuestMigrateTask) OnSrcPrepareCompleteFailed(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	self.TaskFailed(ctx, guest, data.String())
}

func (self *GuestMigrateTask) OnSrcPrepareComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	targetHostId, _ := self.Params.GetString("target_host_id")
	targetHost := models.HostManager.FetchHostById(targetHostId)
	var body *jsonutils.JSONDict
	var hasError bool
	if jsonutils.QueryBoolean(self.Params, "is_local_storage", false) {
		body, hasError = self.localStorageMigrateConf(ctx, guest, targetHost, data)
	} else {
		body, hasError = self.sharedStorageMigrateConf(ctx, guest, targetHost)
	}
	if hasError {
		return
	}
	guestStatus, _ := self.Params.GetString("guest_status")
	if !jsonutils.QueryBoolean(self.Params, "is_rescue_mode", false) && (guestStatus == models.VM_RUNNING || guestStatus == models.VM_SUSPEND) {
		body.Set("live_migrate", jsonutils.JSONTrue)
	}

	headers := http.Header{}
	headers.Set("X-Auth-Token", self.GetUserCred().GetTokenString())
	headers.Set("X-Task-Id", self.GetTaskId())
	headers.Set("X-Region-Version", "v2")

	url := fmt.Sprintf("%s/servers/%s/dest-prepare-migrate", targetHost.ManagerUri, guest.Id)
	self.SetStage("OnMigrateConfAndDiskComplete", nil)
	_, _, err := httputils.JSONRequest(httputils.GetDefaultClient(),
		ctx, "POST", url, headers, body, false)
	if err != nil {
		self.TaskFailed(ctx, guest, err.Error())
	}
}

func (self *GuestMigrateTask) OnMigrateConfAndDiskCompleteFailed(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	targetHostId, _ := self.Params.GetString("target_host_id")
	guest.StartUndeployGuestTask(ctx, self.UserCred, "", targetHostId)
	self.TaskFailed(ctx, guest, data.String())
}

func (self *GuestMigrateTask) OnMigrateConfAndDiskComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	guestStatus, _ := self.Params.GetString("guest_status")
	if !jsonutils.QueryBoolean(self.Params, "is_rescue_mode", false) && (guestStatus == models.VM_RUNNING || guestStatus == models.VM_SUSPEND) {
		// Live migrate
		self.SetStage("OnStartDestComplete", nil)
	} else {
		// Normal migrate
		self.OnNormalMigrateComplete(ctx, guest, data)
	}
}

func (self *GuestMigrateTask) OnNormalMigrateComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	oldHostId := guest.HostId
	self.setGuest(ctx, guest)
	guestStatus, _ := self.Params.GetString("guest_status")
	guest.SetStatus(self.UserCred, guestStatus, "")
	if jsonutils.QueryBoolean(self.Params, "is_rescue_mode", false) {
		guest.StartGueststartTask(ctx, self.UserCred, nil, "")
	}
	self.SetStage("OnUndeployOldHostSucc", nil)
	guest.StartUndeployGuestTask(ctx, self.UserCred, self.GetTaskId(), oldHostId)
}

func (self *GuestMigrateTask) OnUndeployOldHostSucc(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	self.SetStageComplete(ctx, nil)
}

func (self *GuestMigrateTask) sharedStorageMigrateConf(ctx context.Context, guest *models.SGuest, targetHost *models.SHost) (*jsonutils.JSONDict, bool) {
	body := jsonutils.NewDict()
	body.Set("is_local_storage", jsonutils.JSONFalse)
	body.Set("qemu_version", jsonutils.NewString(guest.GetQemuVersion(self.UserCred)))
	targetDesc := guest.GetJsonDescAtHypervisor(ctx, targetHost)
	body.Set("desc", targetDesc)
	return body, false
}

func (self *GuestMigrateTask) localStorageMigrateConf(ctx context.Context,
	guest *models.SGuest, targetHost *models.SHost, data jsonutils.JSONObject) (*jsonutils.JSONDict, bool) {
	body := jsonutils.NewDict()
	if data != nil {
		body.Update(data.(*jsonutils.JSONDict))
	}
	params := jsonutils.NewDict()
	disks := guest.GetDisks()
	for i := 0; i < len(disks); i++ {
		snapshots := models.SnapshotManager.GetDiskSnapshots(disks[i].DiskId)
		snapshotIds := jsonutils.NewArray()
		for j := 0; j < len(snapshots); j++ {
			snapshotIds.Add(jsonutils.NewString(snapshots[j].Id))
		}
		params.Set(disks[i].DiskId, snapshotIds)
	}

	sourceHost := guest.GetHost()
	snapshotsUri := fmt.Sprintf("%s/download/snapshots/", sourceHost.ManagerUri)
	disksUri := fmt.Sprintf("%s/download/disks/", sourceHost.ManagerUri)
	serverUrl := fmt.Sprintf("%s/download/servers/%s", sourceHost.ManagerUri, guest.Id)

	body.Set("src_snapshots", params)
	body.Set("snapshots_uri", jsonutils.NewString(snapshotsUri))
	body.Set("disks_uri", jsonutils.NewString(disksUri))
	body.Set("server_url", jsonutils.NewString(serverUrl))
	body.Set("qemu_version", jsonutils.NewString(guest.GetQemuVersion(self.UserCred)))
	targetDesc := guest.GetJsonDescAtHypervisor(ctx, targetHost)
	jsonDisks, _ := targetDesc.Get("disks")
	if jsonDisks == nil {
		self.TaskFailed(ctx, guest, "Get jsonDisks error")
		return nil, true
	}
	disksDesc, _ := jsonDisks.GetArray()
	if len(disksDesc) == 0 {
		self.TaskFailed(ctx, guest, "Get disksDesc error")
		return nil, true
	}
	targetStorageId, _ := disksDesc[0].GetString("target_storage_id")
	if len(targetStorageId) == 0 {
		self.TaskFailed(ctx, guest, "Get targetStorageId error")
		return nil, true
	}

	targetStorage := targetHost.GetHoststorageOfId(targetStorageId)
	sourceStorage := sourceHost.GetHoststorageOfId(disks[0].GetDisk().StorageId)
	if sourceStorage.MountPoint != targetStorage.MountPoint {
		self.TaskFailed(ctx, guest, fmt.Sprintf("target host %s storage"+
			"mount point is different with source storage", targetHost.Id))
		return nil, true
	}
	body.Set("desc", targetDesc)
	body.Set("is_local_storage", jsonutils.JSONTrue)
	return body, false
}

func (self *GuestLiveMigrateTask) OnStartDestComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	liveMigrateDestPort, err := data.Get("live_migrate_dest_port")
	if err != nil {
		self.TaskFailed(ctx, guest, fmt.Sprintf("Get migrate port error: %s", err))
		return
	}

	targetHostId, _ := self.Params.GetString("target_host_id")
	targetHost := models.HostManager.FetchHostById(targetHostId)

	body := jsonutils.NewDict()
	isLocalStorage, _ := self.Params.Get("is_local_storage")
	body.Set("is_local_storage", isLocalStorage)
	body.Set("live_migrate_dest_port", liveMigrateDestPort)
	body.Set("dest_ip", jsonutils.NewString(targetHost.AccessIp))

	headers := http.Header{}
	headers.Set("X-Auth-Token", self.GetUserCred().GetTokenString())
	headers.Set("X-Task-Id", self.GetTaskId())
	headers.Set("X-Region-Version", "v2")

	host := guest.GetHost()
	url := fmt.Sprintf("%s/servers/%s/live-migrate", host.ManagerUri, guest.Id)
	self.SetStage("OnLiveMigrateComplete", nil)
	_, _, err = httputils.JSONRequest(httputils.GetDefaultClient(),
		ctx, "POST", url, headers, body, false)
	if err != nil {
		self.OnLiveMigrateCompleteFailed(ctx, guest, jsonutils.NewString(err.Error()))
	}
}

func (self *GuestLiveMigrateTask) OnStartDestCompleteFailed(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	targetHostId, _ := self.Params.GetString("target_host_id")
	guest.StartUndeployGuestTask(ctx, self.UserCred, "", targetHostId)
	self.TaskFailed(ctx, guest, data.String())
}

func (self *GuestMigrateTask) setGuest(ctx context.Context, guest *models.SGuest) error {
	targetHostId, _ := self.Params.GetString("target_host_id")
	if jsonutils.QueryBoolean(self.Params, "is_local_storage", false) {
		targetHost := models.HostManager.FetchHostById(targetHostId)
		targetStorage := targetHost.GetLeastUsedStorage(models.STORAGE_LOCAL)
		guestDisks := guest.GetDisks()
		for i := 0; i < len(guestDisks); i++ {
			disk := guestDisks[i].GetDisk()
			disk.GetModelManager().TableSpec().Update(disk, func() error {
				disk.Status = models.DISK_READY
				disk.StorageId = targetStorage.Id
				return nil
			})
			snapshots := models.SnapshotManager.GetDiskSnapshots(disk.Id)
			for _, snapshot := range snapshots {
				snapshot.GetModelManager().TableSpec().Update(snapshot, func() error {
					snapshot.StorageId = targetStorage.Id
					return nil
				})
			}
		}
	}
	oldHost := guest.GetHost()
	oldHost.ClearSchedDescCache()
	err := guest.SetHostId(targetHostId)
	if err != nil {
		return err
	}
	return nil
}

func (self *GuestLiveMigrateTask) OnLiveMigrateCompleteFailed(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	targetHostId, _ := self.Params.GetString("target_host_id")
	guest.StartUndeployGuestTask(ctx, self.UserCred, "", targetHostId)
	self.TaskFailed(ctx, guest, data.String())
}

func (self *GuestLiveMigrateTask) OnLiveMigrateComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	headers := http.Header{}
	headers.Set("X-Auth-Token", self.GetUserCred().GetTokenString())
	headers.Set("X-Task-Id", self.GetTaskId())
	headers.Set("X-Region-Version", "v2")
	body := jsonutils.NewDict()
	body.Set("live_migrate", jsonutils.JSONTrue)
	targetHostId, _ := self.Params.GetString("target_host_id")

	self.SetStage("OnResumeDestGuestComplete", nil)
	targetHost := models.HostManager.FetchHostById(targetHostId)
	url := fmt.Sprintf("%s/servers/%s/resume", targetHost.ManagerUri, guest.Id)
	_, _, err := httputils.JSONRequest(httputils.GetDefaultClient(),
		ctx, "POST", url, headers, body, false)
	if err != nil {
		self.TaskFailed(ctx, guest, err.Error())
	}
}

func (self *GuestLiveMigrateTask) OnResumeDestGuestCompleteFailed(ctx context.Context,
	guest *models.SGuest, data jsonutils.JSONObject) {
	targetHostId, _ := self.Params.GetString("target_host_id")

	guest.StartUndeployGuestTask(ctx, self.UserCred, "", targetHostId)
	self.TaskFailed(ctx, guest, data.String())
}

func (self *GuestLiveMigrateTask) OnResumeDestGuestComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	oldHostId := guest.HostId
	err := self.setGuest(ctx, guest)
	if err != nil {
		self.TaskFailed(ctx, guest, err.Error())
	}
	self.SetStage("OnUndeploySrcGuestComplete", nil)
	err = guest.StartUndeployGuestTask(ctx, self.UserCred, self.GetTaskId(), oldHostId)
	if err != nil {
		self.TaskFailed(ctx, guest, err.Error())
	}
}

func (self *GuestLiveMigrateTask) OnUndeploySrcGuestComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATE, "", self.UserCred)
	status, _ := self.Params.GetString("guest_status")
	if status != models.VM_RUNNING {
		guest.SetStatus(self.UserCred, status, "")
		self.SetStageComplete(ctx, nil)
	} else {
		self.SetStage("OnStartGeustComplete", nil)
		guest.StartGueststartTask(ctx, self.UserCred, nil, self.GetTaskId())
	}
}

func (self *GuestLiveMigrateTask) OnStartGeustComplete(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	self.SetStageComplete(ctx, nil)
}

func (self *GuestMigrateTask) TaskFailed(ctx context.Context, guest *models.SGuest, reason string) {
	status, _ := self.Params.GetString("guest_status")
	if status != models.VM_RUNNING {
		guest.SetStatus(self.UserCred, status, "")
	} else {
		guest.StartGueststartTask(ctx, self.UserCred, nil, "")
	}
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATE_FAIL, reason, self.UserCred)
	self.SetStageFailed(ctx, reason)
	notifyclient.NotifySystemError(guest.Id, guest.Name, models.VM_MIGRATE_FAILED, reason)
}
