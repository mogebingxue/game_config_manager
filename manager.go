package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var instance *ConfigManager
var luaCfgManagerOnce sync.Once

func GetConfigManager() *ConfigManager {
	luaCfgManagerOnce.Do(func() {
		instance = &ConfigManager{
			scanDetail: make(map[string]int64),
			fileStat:   make(map[string]interface{}),
		}
	})
	return instance
}

type IConfig interface {
	GetFileName() string    // 指定 对应配置文件名
	GetResult() interface{} // 指定 数据存储集合
}
type IAfterLoad interface {
	AfterLoad() error
}
type IReloadConfig interface {
	GetReloadResult(alloc bool) interface{}
	OnReloadFinished()
}

type ConfigManager struct {
	basePath   string
	scanDetail map[string]int64       //key：文件名；value：文件修改时间
	fileStat   map[string]interface{} //key：修改过的文件名
}

func (m *ConfigManager) LoadFile(receiver IConfig) {
	file, err := m.loadDataFromFile(receiver.GetFileName(), receiver)
	if err != nil {
		slog.Error("config load failed:", "fileName", receiver.GetFileName(), "err", err)
		return
	}
	err = json.Unmarshal(file, receiver.GetResult())
	if err != nil {
		slog.Error("config load failed:", "fileName", receiver.GetFileName(), "err", err)
		return
	}
	if mod, ok := receiver.GetResult().(IAfterLoad); ok {
		if err := mod.AfterLoad(); err != nil {
			slog.Error("config after load failed:", "fileName", receiver.GetFileName(), "err", err)
			return
		}
	}
}

func (m *ConfigManager) loadDataFromFile(fileName string, receiver interface{}) ([]byte, error) {
	data, err := os.ReadFile(m.basePath + fileName)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, receiver)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (m *ConfigManager) ReloadFile(receiver IConfig) error {
	if IReload, ok := receiver.(IReloadConfig); ok {
		_, err := m.loadDataFromFile(receiver.GetFileName(), IReload.GetReloadResult(true))
		if err != nil {
			return err
		}
		if mod, ok := IReload.GetReloadResult(false).(IAfterLoad); ok {
			if err := mod.AfterLoad(); err != nil {
				return err
			}
		}
		IReload.OnReloadFinished()
		delete(m.fileStat, receiver.GetFileName())
	} else {
		return errors.New("not IReloadConfig")
	}
	return nil
}

func (m *ConfigManager) updateScanDetail(init bool) {
	m.fileStat = make(map[string]interface{})
	filepath.Walk(m.basePath, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".json") {
			return nil
		}
		relPath, err := filepath.Rel(m.basePath, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		timestamp := info.ModTime().Unix()
		if init {
			m.scanDetail[relPath] = timestamp
		} else {
			if oldTimestamp, exist := m.scanDetail[relPath]; exist && oldTimestamp != timestamp {
				m.scanDetail[relPath] = timestamp
				m.fileStat[relPath] = struct{}{}
			}
		}
		return nil
	})
}

func (m *ConfigManager) StartService(path string) {
	m.basePath = path
	instance.updateScanDetail(true)
	go func() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.updateScanDetail(false)
			}
		}
	}()
}
func (m *ConfigManager) IsDirty(fileName string) bool {
	_, exist := m.fileStat[fileName]
	return exist
}
