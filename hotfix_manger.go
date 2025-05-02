package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type HotFixLuaKeeperMgr struct {
	path       string
	scanDetail map[string]int64       //key：文件名；value：文件修改时间
	fileStat   map[string]interface{} //key：修改过的文件名
}

func NewHotFixKeeperMgr(path string) *HotFixLuaKeeperMgr {
	mgr := &HotFixLuaKeeperMgr{
		path:       path,
		scanDetail: make(map[string]int64),
		fileStat:   make(map[string]interface{}),
	}
	mgr.updateScanDetail(true)
	return mgr
}

func (k *HotFixLuaKeeperMgr) updateScanDetail(init bool) {
	k.fileStat = make(map[string]interface{})
	filepath.Walk(k.path, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		relPath, err := filepath.Rel(k.path, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		timestamp := info.ModTime().Unix()
		if init {
			k.scanDetail[relPath] = timestamp
		} else {
			if oldTimestamp, exist := k.scanDetail[relPath]; exist && oldTimestamp != timestamp {
				k.scanDetail[relPath] = timestamp
				k.fileStat[relPath] = struct{}{}
			}
		}
		return nil
	})
}

type HotFixMgr struct {
	started bool
	ch      chan interface{}
	keeper  *HotFixLuaKeeperMgr
}

var hotFixLuaMgrInstance *HotFixMgr
var hotFixLuaMgrOnce sync.Once

func GetHotFixLuaMgr() *HotFixMgr {
	hotFixLuaMgrOnce.Do(func() {
		hotFixLuaMgrInstance = &HotFixMgr{
			started: false,
			ch:      make(chan interface{}),
		}
	})
	return hotFixLuaMgrInstance
}

func (m *HotFixMgr) StartService(path string) {
	m.keeper = NewHotFixKeeperMgr(path)
	go func() {
		m.started = true
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.keeper.updateScanDetail(false)
				m.applyScanDetail()
			case <-m.ch:
				m.keeper.updateScanDetail(false)
				m.applyScanDetail()
			}
		}
	}()
}

func (m *HotFixMgr) applyScanDetail() {
	for file := range m.keeper.fileStat {
		if err := GetConfigManager().ReloadFile(file); err == nil {
			slog.Info("HotFixMgr:applyScanDetail Reload success", "path", m.keeper.path, "filename", file)
		} else {
			slog.Error("HotFixMgr:applyScanDetail Reload failed", "path", m.keeper.path, "filename", file, "error", err)
		}
	}
}

func (m *HotFixMgr) DoService() {
	if m.started {
		go func() {
			m.ch <- true
		}()
	}
}
