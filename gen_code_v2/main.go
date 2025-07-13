package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/types/descriptorpb"
)

// 配置参数
type Config struct {
	ProtoFile string
	Template  string
	Output    string
}

// 注释数据
type CommentData struct {
	LeadingComments         string
	TrailingComments        string
	LeadingDetachedComments []string
}

// 模板数据
type TemplateData struct {
	Package      string
	FileComments CommentData
	Structs      []StructData
	Enums        []EnumData
	FileNames    []string
}

// 结构体数据
type StructData struct {
	Name     string
	Comments CommentData
	Fields   []FieldData
}

// 枚举数据
type EnumData struct {
	Name     string
	Comments CommentData
	Values   []EnumValue
}

// 枚举值
type EnumValue struct {
	Name     string
	Value    int32
	Comments CommentData
}

// 字段数据
type FieldData struct {
	FieldName  string // 字段名
	FieldType  string
	YamlTag    string
	Comments   CommentData
	IsRepeated bool
}

func main() {
	// 解析命令行参数
	cfg := parseFlags()

	// 读取并解析proto文件
	fileDescs, err := parseProto(cfg.ProtoFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing proto: %v\n", err)
		os.Exit(1)
	}

	// 准备模板数据
	data := prepareTemplateData(fileDescs)

	// 执行模板
	if err := renderTemplate(cfg.Template, cfg.Output, data); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering template: %v\n", err)
		os.Exit(1)
	}

	// 执行 go fmt 格式化生成的代码
	if err := goFmtFile(cfg.Output); err != nil {
		fmt.Fprintf(os.Stderr, "Error running go fmt: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully generated Go code and formatted with go fmt!")
}

// 执行 go fmt 格式化文件
func goFmtFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// 解析命令行参数
func parseFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.ProtoFile, "proto", "./gen_code_v2/conf/config.proto", "Path to proto file")
	flag.StringVar(&cfg.Template, "tmpl", "./gen_code_v2/config.tmpl", "Path to template file")
	// 默认输出路径改为空字符串，稍后根据proto文件名生成
	flag.StringVar(&cfg.Output, "out", "", "Output file path (default: same as proto file with .go extension)")
	flag.Parse()

	// 如果没有指定输出路径，则根据proto文件名生成
	if cfg.Output == "" {
		protoDir := filepath.Dir(cfg.ProtoFile)
		protoBase := filepath.Base(cfg.ProtoFile)
		// 移除.proto扩展名，添加.go扩展名
		cfg.Output = filepath.Join(protoDir, strings.TrimSuffix(protoBase, ".proto")+".go")
	}

	return cfg
}

// 解析proto文件并获取文件描述符
func parseProto(protoPath string) ([]*desc.FileDescriptor, error) {
	// 获取绝对路径
	absPath, err := filepath.Abs(protoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 设置解析器
	parser := protoparse.Parser{
		ImportPaths:           []string{filepath.Dir(absPath)},
		IncludeSourceCodeInfo: true,
	}

	// 解析proto文件
	return parser.ParseFiles(filepath.Base(absPath))
}

// 准备模板数据
func prepareTemplateData(fileDescs []*desc.FileDescriptor) TemplateData {
	data := TemplateData{}

	if len(fileDescs) > 0 {
		fd := fileDescs[0]
		// 获取包名
		data.Package = getGoPackage(fd)

		// 获取文件注释
		data.FileComments = getComments(fd.GetSourceInfo())
		data.FileNames = append(data.FileNames, fd.GetName())

		// 处理所有枚举（顶层枚举）
		for _, enum := range fd.GetEnumTypes() {
			data.Enums = append(data.Enums, processEnum(enum))
		}

		// 处理所有消息（顶层消息）
		for _, msg := range fd.GetMessageTypes() {
			data.Structs = append(data.Structs, processMessage(msg))
		}
	}

	return data
}

// 处理消息
func processMessage(msg *desc.MessageDescriptor) StructData {
	sd := StructData{
		Name:     msg.GetName(),
		Comments: getComments(msg.GetSourceInfo()),
	}

	// 处理字段
	for _, field := range msg.GetFields() {
		fieldType := mapProtoType(field)
		fieldName := toCamelCase(field.GetName())
		yamlTag := field.GetJSONName()
		isRepeated := field.IsRepeated()

		// 处理重复字段类型
		if isRepeated {
			if strings.HasPrefix(fieldType, "*") {
				fieldType = "[]" + strings.TrimPrefix(fieldType, "*")
			} else {
				fieldType = "[]" + fieldType
			}
		}

		sd.Fields = append(sd.Fields, FieldData{
			FieldName:  fieldName,
			FieldType:  fieldType,
			YamlTag:    yamlTag,
			Comments:   getComments(field.GetSourceInfo()),
			IsRepeated: isRepeated,
		})
	}

	return sd
}

// 处理枚举
func processEnum(enum *desc.EnumDescriptor) EnumData {
	ed := EnumData{
		Name:     enum.GetName(),
		Comments: getComments(enum.GetSourceInfo()),
	}

	for _, value := range enum.GetValues() {
		ed.Values = append(ed.Values, EnumValue{
			Name:     value.GetName(),
			Value:    value.GetNumber(),
			Comments: getComments(value.GetSourceInfo()),
		})
	}

	return ed
}

// 获取Go包名
func getGoPackage(fd *desc.FileDescriptor) string {
	options := fd.AsFileDescriptorProto().GetOptions()
	if options != nil {
		goPackage := options.GetGoPackage()
		if goPackage != "" {
			// 提取包名（可能包含路径）
			if parts := strings.Split(goPackage, ";"); len(parts) > 1 {
				return parts[1]
			}
			if parts := strings.Split(goPackage, "/"); len(parts) > 0 {
				return parts[len(parts)-1]
			}
			return goPackage
		}
	}
	return "main"
}

// 获取注释信息
func getComments(info *descriptorpb.SourceCodeInfo_Location) CommentData {
	if info == nil {
		return CommentData{}
	}

	// 清理并格式化注释
	clean := func(s string) string {
		s = strings.TrimSpace(s)
		return s
	}

	return CommentData{
		LeadingComments:         clean(info.GetLeadingComments()),
		TrailingComments:        clean(info.GetTrailingComments()),
		LeadingDetachedComments: info.GetLeadingDetachedComments(),
	}
}

// 映射proto类型到Go类型
func mapProtoType(field *desc.FieldDescriptor) string {
	switch field.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "int64"

	case descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "int32"

	case descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return "uint64"

	case descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return "uint32"

	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "float64"

	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float32"

	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"

	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return "string"

	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return "[]byte"

	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		return field.GetEnumType().GetName()

	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "*" + field.GetMessageType().GetName()

	default:
		return "interface{}"
	}
}

// 转换为驼峰命名
func toCamelCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// 渲染模板
func renderTemplate(tplPath, outputPath string, data TemplateData) error {
	// 读取模板文件
	tplContent, err := os.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// 创建模板
	tmpl, err := template.New("generated").
		Funcs(template.FuncMap{
			"toLower": strings.ToLower,
			"hasComments": func(cd CommentData) bool {
				return cd.LeadingComments != "" || cd.TrailingComments != "" || len(cd.LeadingDetachedComments) > 0
			},
			"formatDetachedComments": func(comments []string) string {
				var result strings.Builder
				for _, c := range comments {
					c = strings.TrimSpace(c)
					if c == "" {
						continue
					}
					lines := strings.Split(c, "\n")
					for _, line := range lines {
						if line = strings.TrimSpace(line); line != "" {
							result.WriteString("// " + line + "\n")
						}
					}
					result.WriteString("//\n") // 分离注释之间添加空行
				}
				return strings.TrimSpace(result.String())
			},
			"formatLeadingComments": func(comment string) string {
				if comment == "" {
					return ""
				}
				return "// " + strings.ReplaceAll(strings.TrimSpace(comment), "\n", "\n// ")
			},
			"formatTrailingComments": func(comment string) string {
				if comment == "" {
					return ""
				}
				return "// " + strings.ReplaceAll(strings.TrimSpace(comment), "\n", "\n// ")
			},
		}).
		Parse(string(tplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// 创建输出文件
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// 执行模板
	if err := tmpl.Execute(outputFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
