package main

import (
	"encoding/json"
	"fmt"
	"github.com/mogebingxue/game_config_manager/utils"
	"log"
	"os"
	"path/filepath"
)

func GenTableTrees(confMap map[string]*utils.Conf, alltypMap map[string]map[string]utils.Meta, dataMap map[string]map[string]map[string]any) {
	for pack, conf := range confMap {
		tableTree := make(map[string]*TableTree)
		typMap := alltypMap[pack]
		for _, table := range conf.Tables {
			tableTree[table.Name] = InitTableTree(pack, utils.Table(table), typMap)
		}
		tableTreeMap[pack] = tableTree
	}
	for pack, packTableTree := range tableTreeMap {
		tableDataMap, ok := dataMap[pack]
		if !ok {
			continue
		}
		for _, tableTree := range packTableTree {
			tableData, ok := tableDataMap[tableTree.Name]
			if !ok {
				continue
			}
			typMap := alltypMap[pack]
			for _, node := range tableTree.Nodes {
				val, ok := tableData[node.Name]
				if !ok {
					continue
				}
				FillNodeData(node, typMap, val)
			}
		}
	}
}

func InitTableTree(pack string, table utils.Table, typMap map[string]utils.Meta) *TableTree {
	tableTree := &TableTree{}
	tableTree.Pack = pack
	tableTree.Name = table.Name
	tableTree.Alias = table.Alias
	tableTree.Nodes = make([]*TreeNode, len(table.Vars))
	for i, structVar := range table.Vars {
		tableTree.Nodes[i] = InitTableNode(1, structVar, typMap)
	}
	return tableTree
}

func InitTableNode(level int, structVar utils.StructVar, typMap map[string]utils.Meta) *TreeNode {
	if level > 10 {
		return nil
	}
	treeNode := &TreeNode{}
	treeNode.Name = structVar.Name
	treeNode.Alias = structVar.Alias
	treeNode.Level = level + 1
	treeNode.Typ = structVar.Typ
	switch structVar.Typ {
	case "int", "string", "bool":
	case "list", "map":
		treeNode.ValTyp = structVar.ValueType
		switch structVar.ValueType {
		case "", "list", "map":
			return nil
		case "int", "string", "bool":
		default:
			meta, ok := typMap[structVar.ValueType]
			if !ok || meta.Typ == utils.TABLE {
				return nil
			}
		}
	default:
		meta, ok := typMap[structVar.Typ]
		if !ok || meta.Typ == utils.TABLE {
			return nil
		}
		if meta.Typ == utils.STRUCT {
			subStruct := meta.Meta.(*utils.Struct)
			treeNode.Nodes = make([]*TreeNode, len(subStruct.Vars))
			for i, structVar := range subStruct.Vars {
				treeNode.Nodes[i] = InitTableNode(level+1, structVar, typMap)
			}
		}
	}
	return treeNode
}

func FillNodeData(node *TreeNode, typMap map[string]utils.Meta, val any) {
	switch node.Typ {
	case "int":
		sVal, ok := val.(float64)
		if !ok {
			node.Val = 1
		} else {
			node.Val = int(sVal)
		}
	case "string":
		sVal, ok := val.(string)
		if !ok {
			node.Val = ""
		} else {
			node.Val = sVal
		}
	case "bool":
		sVal, ok := val.(bool)
		if !ok {
			node.Val = false
		} else {
			node.Val = sVal
		}
	case "list":
		FillList(node, typMap, val)
	case "map":
		FillMap(node, typMap, val)
	default:
		meta, ok := typMap[node.Typ]
		if !ok || meta.Typ == utils.TABLE {
			return
		}
		if meta.Typ == utils.STRUCT {
			for _, subNode := range node.Nodes {
				valSub, ok := val.(map[string]any)[subNode.Name]
				if !ok {
					continue
				}
				FillNodeData(subNode, typMap, valSub)
			}
		} else if meta.Typ == utils.ENUM {
			sVal, ok := val.(float64)
			if !ok {
				node.Val = 1
			} else {
				node.Val = int(sVal)
			}
		}

	}
}

func FillList(node *TreeNode, typMap map[string]utils.Meta, val any) {
	list := val.([]any)
	nodeList := make([]*TreeNode, len(list))
	for i, _ := range list {
		structVar := utils.StructVar{
			Name:  fmt.Sprintf("%s_%d", node.Name, i),
			Alias: fmt.Sprintf("%s_%d", node.Alias, i),
			Typ:   node.ValTyp,
		}
		nodeList[i] = InitTableNode(node.Level, structVar, typMap)
		FillNodeData(nodeList[i], typMap, list[i])
	}
	node.Val = nodeList
}

func FillMap(node *TreeNode, typMap map[string]utils.Meta, val any) {
	valMap := val.(map[string]any)
	nodeList := make([]*TreeNode, len(valMap))
	i := 0
	for k, _ := range valMap {
		structVar := utils.StructVar{
			Name:  fmt.Sprintf("%s_%d", node.Name, i),
			Alias: fmt.Sprintf("%s_%d", node.Alias, i),
			Typ:   node.ValTyp,
		}
		nodeList[i] = InitTableNode(node.Level, structVar, typMap)
		FillNodeData(nodeList[i], typMap, valMap[k])
		nodeList[i].Key = k
		i++
	}
	node.Val = nodeList
}

var tableTreeMap = make(map[string]map[string]*TableTree)

type TableTree struct {
	Pack  string
	Name  string
	Alias string
	Nodes []*TreeNode
}

type TreeNode struct {
	Level   int //深度，不允许太多层
	Name    string
	Alias   string
	Typ     string
	ValTyp  string
	Val     any
	Nodes   []*TreeNode
	Key     string //map类型的子节点的key
	Deleted bool   //标记list元素被删除
}

func (t *TableTree) Save() {
	//如果文件夹不存在创建文件夹
	if _, err := os.Stat(baseDtaPath + t.Pack); os.IsNotExist(err) {
		os.Mkdir(t.Pack, 0755)
	}
	//创建文件，如果存在就覆盖
	file, err := os.Create(filepath.Join(baseDtaPath, t.Pack, t.Name) + ".json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	file.Write(t.ToJson())
}

func (t *TableTree) ToJson() []byte {
	m := make(map[string]any)
	for _, node := range t.Nodes {
		nodeVal, ok := node.ToJson(utils.AllTypMap[t.Pack])
		if nodeVal != nil && ok {
			m[node.Name] = nodeVal
		}
	}
	data, _ := json.MarshalIndent(m, "", "    ")
	return data
}

func (n *TreeNode) ToJson(typMap map[string]utils.Meta) (any, bool) {
	switch n.Typ {
	case "int", "string", "bool":
		return n.Val, true
	case "list":
		list, ok := n.Val.([]*TreeNode)
		if !ok {
			return nil, false
		}
		valList := make([]any, 0, len(list))
		for _, v := range list {
			if !v.Deleted {
				valList = append(valList, v.Val)
			}
		}
		switch n.ValTyp {
		case "int", "string", "bool":
			return valList, true
		default:
			meta, ok := typMap[n.ValTyp]
			if !ok || meta.Typ == utils.TABLE {
				return nil, false
			}
			if meta.Typ == utils.STRUCT {
				for i, child := range valList {
					childNode, ok := child.(*TreeNode)
					if !ok {
						return nil, false
					}
					nodeVal, ok := childNode.ToJson(typMap)
					if nodeVal != nil && ok {
						valList[i] = nodeVal
					}
				}
				return valList, true
			} else if meta.Typ == utils.ENUM {
				return valList, true
			}
		}
	case "map":
		nodeList, ok := n.Val.([]*TreeNode)
		if !ok {
			return nil, false
		}
		valMap := make(map[string]any)
		for _, v := range nodeList {
			if !v.Deleted && v.Key != "" {
				valMap[v.Key] = v.Val
			}
		}
		switch n.ValTyp {
		case "int", "string", "bool":
			return valMap, true
		default:
			meta, ok := typMap[n.ValTyp]
			if !ok || meta.Typ == utils.TABLE {
				return nil, false
			}
			if meta.Typ == utils.STRUCT {
				for k, child := range valMap {
					childNode, ok := child.(*TreeNode)
					if !ok {
						return nil, false
					}
					nodeVal, ok := childNode.ToJson(typMap)
					if nodeVal != nil && ok {
						valMap[k] = nodeVal
					}
				}
				return valMap, true
			} else if meta.Typ == utils.ENUM {
				return valMap, true
			}
		}
	default:
		meta, ok := typMap[n.Typ]
		if !ok || meta.Typ == utils.TABLE {
			return nil, false
		}
		if meta.Typ == utils.STRUCT {
			subNodes := make(map[string]any)
			for _, child := range n.Nodes {
				nodeVal, ok := child.ToJson(typMap)
				if nodeVal != nil && ok {
					subNodes[child.Name] = nodeVal
				}
			}
			return subNodes, true
		} else if meta.Typ == utils.ENUM {
			return n.Val, true
		}
	}
	return nil, false
}
