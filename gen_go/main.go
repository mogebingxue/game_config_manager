package main

import (
	"encoding/xml"
	"fmt"
	"github.com/mogebingxue/game_config_manager"
	"github.com/mogebingxue/game_config_manager/utils"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	cfg, err := config.LoadConfig("./conf.yaml")
	if err != nil {
		slog.Error("gen go", "err", err)
		return
	}
	GenGoConf(cfg.MetadataPath, cfg.ConfGoPath)
}

func GenGoConf(srcPath, outPath string) {
	filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
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
		// 打印文件路径
		genConf(path, outPath)

		return nil
	})

}

func genConf(srcFilePath, outPath string) {
	xmlFile, err := os.Open(srcFilePath)
	if err != nil {
		slog.Error("Error opening input file:", "err", err)
		return
	}
	content, err := io.ReadAll(xmlFile)
	if err != nil {
		slog.Info("Error reading input file:", "err", err)
		return
	}
	conf := &utils.Conf{}
	err = xml.Unmarshal(content, conf)
	if err != nil {
		slog.Info("Error:", "err", err)
		os.Exit(1)
	}
	err, _ = utils.CheckConfValid(conf)
	if err != nil {
		slog.Info("Error:", "err", err)
		os.Exit(1)
	}
	for _, v := range conf.Enums {
		fileName, writeContent := GenEnum(conf.Package, conf.Alias, &v)
		WriteToFile(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName), writeContent)
		GoFmt(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName))
	}
	for _, v := range conf.Structs {
		fileName, writeContent := GenStruct(conf.Package, conf.Alias, &v)
		WriteToFile(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName), writeContent)
		GoFmt(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName))
	}
	for _, v := range conf.Tables {
		fileName, writeContent := GenTable(conf.Package, conf.Alias, &v)
		WriteToFile(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName), writeContent)
		GoFmt(fmt.Sprintf("%s%s/%s.go", outPath, conf.Package, fileName))
	}

	slog.Info("gen config", "file:", srcFilePath)
}

func GoFmt(fileName string) {
	//格式化代码
	cmd := exec.Command("go", "fmt", fileName)
	err := cmd.Run()
	if err != nil {
		slog.Info("Error formatting file:", "err", err)
		return
	}
}
func WriteToFile(fileName, content string) {
	//创建文件夹
	dir := fileName[0:strings.LastIndex(fileName, "/")]
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		slog.Info("Error creating dir:", "err", err)
		return
	}
	file, err := os.Create(fileName)
	if err != nil {
		slog.Info("Error creating file:", "err", err)
		return
	}
	defer file.Close()
	_, err = file.WriteString(content)
	if err != nil {
		slog.Info("Error writing file:", "err", err)
		return
	}
}

func GenStruct(packageName, packageAlias string, tStruct *utils.Struct) (string, string) {
	var buffer strings.Builder
	buffer.WriteString(GetPkgStr(packageName, packageAlias))
	buffer.WriteString(GenStructWithoutPackage(tStruct))
	return tStruct.Name, buffer.String()
}

func GenStructWithoutPackage(tStruct *utils.Struct) string {
	var buffer strings.Builder
	buffer.WriteString(fmt.Sprintf("// %s\n", tStruct.Alias))
	buffer.WriteString(fmt.Sprintf("type %s struct {\n", tStruct.Name))
	for _, v := range tStruct.Vars {
		switch v.Typ {
		case "int":
			buffer.WriteString(fmt.Sprintf("\t%s int // %s\n", v.Name, v.Alias))
		case "list":
			switch v.ValueType {
			default:
				buffer.WriteString(fmt.Sprintf("\t%s []%s // %s\n", v.Name, v.ValueType, v.Alias))
			}
		case "map":
			switch v.ValueType {
			default:
				buffer.WriteString(fmt.Sprintf("\t%s map[string]%s // %s\n", v.Name, v.ValueType, v.Alias))
			}
		default:
			buffer.WriteString(fmt.Sprintf("\t%s %s // %s\n", v.Name, v.Typ, v.Alias))
		}

	}
	buffer.WriteString(fmt.Sprintf("}\n"))
	return buffer.String()
}

func GenTable(packageName, packageAlias string, tStruct *utils.Struct) (string, string) {
	fileName, structContent := tStruct.Name, GenStructWithoutPackage(tStruct)
	var buffer strings.Builder

	buffer.WriteString(GetPkgStr(packageName, packageAlias))
	//导入包
	buffer.WriteString(fmt.Sprintf("import \"github.com/mogebingxue/game_config_manager\"\n\n"))
	//生成结构
	buffer.WriteString(structContent)
	//生成变量
	buffer.WriteString(fmt.Sprintf("\nvar %s *%s\n", FirstToLower(fileName), fileName))
	buffer.WriteString(fmt.Sprintf("var reload%s *%s\n", fileName, fileName))
	//生成基础接口
	buffer.WriteString(fmt.Sprintf("\nfunc (cfg *%s) GetFileName() string {\n", fileName))
	buffer.WriteString(fmt.Sprintf("\treturn \"%s/%s.json\"\n", packageName, fileName))
	buffer.WriteString(fmt.Sprintf("}\n"))
	buffer.WriteString(fmt.Sprintf("\nfunc (cfg *%s) GetResult() interface{} {\n", fileName))
	buffer.WriteString(fmt.Sprintf("\treturn %s\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("}\n"))
	buffer.WriteString(fmt.Sprintf("\nfunc (cfg *%s) GetReloadResult(alloc bool) interface{} {\n", fileName))
	buffer.WriteString(fmt.Sprintf("\tif alloc || reload%s == nil {\n", fileName))
	buffer.WriteString(fmt.Sprintf("\t\treload%s = new(%s)\n", fileName, fileName))
	buffer.WriteString(fmt.Sprintf("\t}\n"))
	buffer.WriteString(fmt.Sprintf("\treturn reload%s\n", fileName))
	buffer.WriteString(fmt.Sprintf("}\n"))
	buffer.WriteString(fmt.Sprintf("\nfunc (cfg *%s) OnReloadFinished() {\n", fileName))
	buffer.WriteString(fmt.Sprintf("\t%s = reload%s\n", FirstToLower(fileName), fileName))
	buffer.WriteString(fmt.Sprintf("}\n"))
	//生成获取接口
	buffer.WriteString(fmt.Sprintf("\nfunc Get%s() *%s {\n", fileName, fileName))
	buffer.WriteString(fmt.Sprintf("\tif %s== nil {\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("\t\t%s = &%s{}\n", FirstToLower(fileName), fileName))
	buffer.WriteString(fmt.Sprintf("\t\tconfig.GetConfigManager().LoadFile(%s)\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("\t}\n"))
	buffer.WriteString(fmt.Sprintf("\tif config.GetConfigManager().IsDirty(%s.GetFileName()){\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("\t\tconfig.GetConfigManager().ReloadFile(%s)\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("\t}\n"))
	buffer.WriteString(fmt.Sprintf("\treturn %s\n", FirstToLower(fileName)))
	buffer.WriteString(fmt.Sprintf("}\n"))
	return fileName, buffer.String()
}

func FirstToLower(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToLower(str[0:1]) + str[1:]
}

func GetPkgStr(packageName, packageAlias string) string {
	return fmt.Sprintf("// Code generated by gen_cfg_go. DO NOT EDIT.\n\n//%s\n package %s\n\n ", packageAlias, packageName)
}

func GenEnum(packageName, packageAlias string, enum *utils.Enum) (string, string) {
	var buffer strings.Builder
	buffer.WriteString(GetPkgStr(packageName, packageAlias))
	buffer.WriteString(fmt.Sprintf("// %s\n", enum.Alias))
	buffer.WriteString(fmt.Sprintf("type %s uint8\n\n", enum.Name))
	buffer.WriteString(fmt.Sprintf("const (\n"))
	for _, v := range enum.Vars {
		buffer.WriteString(fmt.Sprintf("\t%s_%s %s = %s // %s\n", enum.Name, v.Name, enum.Name, v.Default, v.Alias))
	}
	buffer.WriteString(fmt.Sprintf(")\n"))
	return enum.Name, buffer.String()
}
