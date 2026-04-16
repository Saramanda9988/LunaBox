package service

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/common/vo"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/service/cloudsync"
	"sync"
	"time"
)

const (
	cloudSyncStateIdle    = "idle"
	cloudSyncStateSyncing = "syncing"
	cloudSyncStateSuccess = "success"
	cloudSyncStateFailed  = "failed"
)

type CloudSyncService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig

	mu            sync.Mutex
	syncing       bool
	schedulerStop chan struct{}
	schedulerDone chan struct{}
}

func NewCloudSyncService() *CloudSyncService {
	return &CloudSyncService{}
}

func (s *CloudSyncService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

func (s *CloudSyncService) GetCloudSyncStatus() vo.CloudSyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return vo.CloudSyncStatus{
		Enabled:        s.config.CloudSyncEnabled,
		Configured:     cloudprovider.IsConfigured(s.config),
		Syncing:        s.syncing,
		LastSyncTime:   s.config.LastCloudSyncTime,
		LastSyncStatus: s.config.LastCloudSyncStatus,
		LastSyncError:  s.config.LastCloudSyncError,
	}
}

func (s *CloudSyncService) SyncNow() (vo.CloudSyncStatus, error) {
	s.mu.Lock()
	if s.syncing {
		status := s.currentStatusLocked()
		s.mu.Unlock()
		return status, nil
	}
	s.syncing = true
	s.config.LastCloudSyncStatus = cloudSyncStateSyncing
	s.config.LastCloudSyncError = ""
	_ = appconf.SaveConfig(s.config)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.mu.Unlock()
	}()

	if !s.config.CloudSyncEnabled {
		return s.finishSync(cloudSyncStateIdle, "", nil)
	}

	provider, err := cloudprovider.NewCloudProvider(s.ctx, s.config)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	helper := cloudsync.NewHelper(s.ctx, s.db, s.config)

	localState, err := helper.BuildLocalState()
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	remoteSnapshot, remoteExists, err := helper.LoadRemoteSnapshot(provider)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	merged := helper.MergeSnapshots(localState.Snapshot, remoteSnapshot, remoteExists)
	coverURLs, err := helper.ReconcileCoverAssets(provider, localState, remoteSnapshot, remoteExists, merged)
	if err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	if err := helper.ApplyMergedSnapshot(merged, coverURLs); err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	if err := helper.SaveRemoteSnapshot(provider, merged); err != nil {
		return s.finishSync(cloudSyncStateFailed, err.Error(), err)
	}

	return s.finishSync(cloudSyncStateSuccess, "", nil)
}

func (s *CloudSyncService) currentStatusLocked() vo.CloudSyncStatus {
	return vo.CloudSyncStatus{
		Enabled:        s.config.CloudSyncEnabled,
		Configured:     cloudprovider.IsConfigured(s.config),
		Syncing:        s.syncing,
		LastSyncTime:   s.config.LastCloudSyncTime,
		LastSyncStatus: s.config.LastCloudSyncStatus,
		LastSyncError:  s.config.LastCloudSyncError,
	}
}

func (s *CloudSyncService) finishSync(state, lastError string, syncErr error) (vo.CloudSyncStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.LastCloudSyncStatus = state
	s.config.LastCloudSyncError = lastError
	if state == cloudSyncStateSuccess {
		s.config.LastCloudSyncTime = time.Now().Format(time.RFC3339)
	}
	_ = appconf.SaveConfig(s.config)

	return s.currentStatusLocked(), syncErr
}

func (s *CloudSyncService) RunStartupSync() {
	if !s.config.CloudSyncEnabled || !cloudprovider.IsConfigured(s.config) {
		return
	}

	go func() {
		if _, err := s.SyncNow(); err != nil {
			applog.LogWarningf(s.ctx, "CloudSyncService.RunStartupSync: sync failed: %v", err)
		}
	}()
}

func (s *CloudSyncService) StartScheduledSync() {
	s.mu.Lock()
	if s.schedulerStop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	s.schedulerStop = stop
	s.schedulerDone = done
	s.mu.Unlock()

	go func() {
		defer close(done)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var nextSyncAt time.Time
		var lastInterval time.Duration

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if !s.config.CloudSyncEnabled || !cloudprovider.IsConfigured(s.config) {
					nextSyncAt = time.Time{}
					lastInterval = 0
					continue
				}

				interval := s.syncInterval()
				if nextSyncAt.IsZero() || interval != lastInterval {
					nextSyncAt = time.Now().Add(interval)
					lastInterval = interval
					continue
				}

				if time.Now().Before(nextSyncAt) {
					continue
				}

				if _, err := s.SyncNow(); err != nil {
					applog.LogWarningf(s.ctx, "CloudSyncService.StartScheduledSync: sync failed: %v", err)
				}

				interval = s.syncInterval()
				nextSyncAt = time.Now().Add(interval)
				lastInterval = interval
			}
		}
	}()
}

func (s *CloudSyncService) StopScheduledSync() {
	s.mu.Lock()
	stop := s.schedulerStop
	done := s.schedulerDone
	if stop == nil {
		s.mu.Unlock()
		return
	}
	s.schedulerStop = nil
	s.schedulerDone = nil
	s.mu.Unlock()

	close(stop)
	if done != nil {
		<-done
	}
}

func (s *CloudSyncService) syncInterval() time.Duration {
	seconds := s.config.CloudSyncIntervalSec
	if seconds <= 0 {
		seconds = 60
	}
	if seconds < 15 {
		seconds = 15
	}
	return time.Duration(seconds) * time.Second
}
