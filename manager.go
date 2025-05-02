package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
)

var instance *ConfigManager
var luaCfgManagerOnce sync.Once

func GetConfigManager() *ConfigManager {
	luaCfgManagerOnce.Do(func() {
		instance = &ConfigManager{
			config: make(map[string]IConfig),
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
	basePath string
	config   map[string]IConfig
}

func (m *ConfigManager) LoadAll(basePath string) {
	m.basePath = basePath
	for _, v := range m.config {
		file, err := m.loadDataFromFile(v.GetFileName(), v)
		if err != nil {
			slog.Error("config load failed:", "fileName", v.GetFileName(), "err", err)
			continue
		}
		err = json.Unmarshal(file, v.GetResult())
		if err != nil {
			slog.Error("config load failed:", "fileName", v.GetFileName(), "err", err)
			continue
		}
		if mod, ok := v.GetResult().(IAfterLoad); ok {
			if err := mod.AfterLoad(); err != nil {
				slog.Error("config after load failed:", "fileName", v.GetFileName(), "err", err)
				continue
			}
		}
	}
}

func (m *ConfigManager) Register(cfg IConfig) {
	m.config[cfg.GetFileName()] = cfg
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

func (m *ConfigManager) ReloadFile(fileName string) error {
	v, ok := m.config[fileName]
	if !ok {
		return errors.New("config file not exist")
	}
	if IReload, ok := v.(IReloadConfig); ok {
		_, err := m.loadDataFromFile(v.GetFileName(), IReload.GetReloadResult(true))
		if err != nil {
			return err
		}
		if mod, ok := IReload.GetReloadResult(false).(IAfterLoad); ok {
			if err := mod.AfterLoad(); err != nil {
				return err
			}
		}
		IReload.OnReloadFinished()
	} else {
		return errors.New("not ILuaReloadConfig")
	}
	return nil
}
