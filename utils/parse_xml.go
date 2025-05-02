package utils

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func LoadAllConfigs(path string) error {
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		//只读取后缀为xml的
		if !strings.HasSuffix(path, ".xml") {
			return nil
		}
		// 忽略目录
		if info == nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		waitLoadConfMap.Add(1)
		go func() {
			conf, typMap := LoadMetadata(path)
			slog.Debug("LoadMetadata", conf)
			confMapMutex.Lock()
			confMap[conf.Package] = conf
			AllTypMap[conf.Package] = typMap
			confMapMutex.Unlock()
			waitLoadConfMap.Done()
		}()
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func WaitLoadConfMap() {
	waitLoadConfMap.Wait()
}

var waitLoadConfMap sync.WaitGroup
var confMapMutex sync.RWMutex
var confMap = make(map[string]*Conf)

func GetConfMap() map[string]*Conf {
	confMapMutex.RLock()
	defer confMapMutex.RUnlock()
	return confMap
}

var AllTypMap = make(map[string]map[string]Meta)

func LoadMetadata(path string) (*Conf, map[string]Meta) {
	xmlFile, err := os.Open(path)
	if err != nil {
		slog.Error("Error opening input file:", err)
		os.Exit(1)
	}
	defer xmlFile.Close()
	content, err := io.ReadAll(xmlFile)
	if err != nil {
		slog.Error("Error reading input file:", err)
		os.Exit(1)
	}
	conf := &Conf{}
	err = xml.Unmarshal(content, conf)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	err, typMap := CheckConfValid(conf)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	return conf, typMap
}

type Conf struct {
	Package string   `xml:"package,attr"`
	Alias   string   `xml:"alias,attr"`
	Enums   []Enum   `xml:"enum"`
	Structs []Struct `xml:"struct"`
	Tables  []Struct `xml:"table"`
}

type Table Struct

type Enum struct {
	Name  string    `xml:"name,attr"`
	Alias string    `xml:"alias,attr"`
	Vars  []EnumVar `xml:"var"`
}

type EnumVar struct {
	Name    string `xml:"name,attr"`
	Default string `xml:"default,attr"`
	Alias   string `xml:"alias,attr"`
}
type Struct struct {
	Name  string      `xml:"name,attr"`
	Alias string      `xml:"alias,attr"`
	Vars  []StructVar `xml:"var"`
}

type StructVar struct {
	Name      string `xml:"name,attr"`
	Typ       string `xml:"type,attr"`
	ValueType string `xml:"valueType,attr"`
	Alias     string `xml:"alias,attr"`
}

type Meta struct {
	Typ  META_TYPE
	Meta interface{}
}

type META_TYPE int32

const (
	ENUM   META_TYPE = 1
	STRUCT META_TYPE = 2
	TABLE  META_TYPE = 3
)

func CheckConfValid(conf *Conf) (error, map[string]Meta) {
	typMap := make(map[string]Meta) //name isTable
	for _, v := range conf.Enums {
		if _, ok := typMap[v.Name]; ok {
			return errors.New("duplicate enum name" + v.Name), nil
		}
		typMap[v.Name] = Meta{
			Typ:  ENUM,
			Meta: &v,
		}
	}
	for _, v := range conf.Structs {
		if _, ok := typMap[v.Name]; ok {
			return errors.New("duplicate struct name" + v.Name), nil
		}
		typMap[v.Name] = Meta{
			Typ:  STRUCT,
			Meta: &v,
		}
	}
	for _, v := range conf.Tables {
		if _, ok := typMap[v.Name]; ok {
			return errors.New("duplicate table name" + v.Name), nil
		}
		typMap[v.Name] = Meta{
			Typ:  TABLE,
			Meta: &v,
		}
	}
	err := checkSubType(typMap, conf.Structs)
	if err != nil {
		return err, nil
	}
	err = checkSubType(typMap, conf.Tables)
	if err != nil {
		return err, nil
	}
	return nil, typMap
}

func checkSubType(typMap map[string]Meta, structs []Struct) error {
	for _, v := range structs {
		for _, structVar := range v.Vars {
			switch structVar.Typ {
			case "int", "string", "bool":
			case "list", "map":
				switch structVar.ValueType {
				case "":
					return errors.New("valueType is empty " + v.Name + "." + structVar.Name + " type:" + structVar.Typ)
				case "int", "string", "bool", "list", "map":
				default:
					meta, ok := typMap[structVar.ValueType]
					if !ok {
						return errors.New("type not found " + v.Name + "." + structVar.Name + " type:" + structVar.Typ)
					}
					if meta.Typ == TABLE {
						return errors.New("type is table" + v.Name + "." + structVar.Name + " type:" + structVar.Typ)
					}
				}
			default:
				meta, ok := typMap[structVar.Typ]
				if !ok {
					return errors.New("type not found " + v.Name + "." + structVar.Name)
				}
				if meta.Typ == TABLE {
					return errors.New("type is table " + v.Name + "." + structVar.Name)
				}
			}
		}
	}
	return nil
}
