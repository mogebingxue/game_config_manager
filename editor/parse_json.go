package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var baseDtaPath = ""

func LoadAllJsonData(path string) error {
	baseDtaPath = path
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		//只读取后缀为json的
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		// 忽略目录
		if info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		waitLoadJsonDataMap.Add(1)
		pack := filepath.Base(filepath.Dir(path))
		go func() {
			data := LoadJsonData(path)
			slog.Debug("LoadData", data)
			jsonDataMapMutex.Lock()
			if jsonDataMap[pack] == nil {
				jsonDataMap[pack] = make(map[string]map[string]any)
			}
			ext := filepath.Ext(info.Name())
			// 去掉后缀部分作为
			jsonDataMap[pack][strings.TrimSuffix(info.Name(), ext)] = data
			jsonDataMapMutex.Unlock()
			waitLoadJsonDataMap.Done()
		}()
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
func WaitLoadJsonDataMap() {
	waitLoadJsonDataMap.Wait()
}

var waitLoadJsonDataMap sync.WaitGroup
var jsonDataMapMutex sync.Mutex
var jsonDataMap = make(map[string]map[string]map[string]any)

func LoadJsonData(path string) map[string]any {
	jsonFile, err := os.Open(path)
	if err != nil {
		slog.Error("Error opening input file:", err)
		os.Exit(1)
	}
	defer jsonFile.Close()
	content, err := io.ReadAll(jsonFile)
	if err != nil {
		slog.Error("Error reading input file:", err)
		os.Exit(1)
	}
	res := make(map[string]any)
	err = json.Unmarshal(content, &res)
	if err != nil {
		slog.Error("Error reading input file:", err)
		os.Exit(1)
	}
	return res
}
