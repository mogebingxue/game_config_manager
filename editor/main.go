package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/mogebingxue/game_config_manager"
	"github.com/mogebingxue/game_config_manager/utils"
	"sort"
	"strconv"
)

var wCtrl = container.NewMultipleWindows()

const (
	High  = float32(680)
	Width = float32(1080)
)

func main() {
	cfg, err := config.LoadConfig("./conf.yaml")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = utils.LoadAllConfigs(cfg.MetadataPath)
	if err != nil {
		return
	}
	err = LoadAllJsonData(cfg.DataPath)
	if err != nil {
		return
	}
	utils.WaitLoadConfMap()
	WaitLoadJsonDataMap()
	GenTableTrees(utils.GetConfMap(), utils.AllTypMap, jsonDataMap)
	a := app.New()
	w := a.NewWindow("Config Editor")
	TableDisplay := container.NewVBox()
	TableDisplayScroll := container.NewStack(container.NewVScroll(TableDisplay), wCtrl)
	// 布局设置
	mainContainer := container.NewBorder(
		container.NewHBox(
			widget.NewToolbar(widget.NewToolbarAction(theme.DocumentSaveIcon(), func() { selectTable.Save() })),
			InitSelect(TableDisplay)),
		nil, nil, nil,
		TableDisplayScroll,
	)
	w.SetContent(mainContainer)
	// 设置布局
	w.Resize(fyne.NewSize(Width, High))
	w.ShowAndRun()
}

var selectTable *TableTree

func InitSelect(tableDisplay *fyne.Container) *fyne.Container {
	packSelect := widget.NewSelect([]string{}, nil)
	tableSelect := widget.NewSelect([]string{}, nil)
	packSelect.PlaceHolder = "请选择包"
	tableSelect.PlaceHolder = "请选择表"
	// 初始化第一级选项
	var pkgArr []string
	alias2pkg := make(map[string]string)
	for k := range tableTreeMap {
		cm := utils.GetConfMap()
		alias2pkg[cm[k].Alias] = k
		pkgArr = append(pkgArr, cm[k].Alias)
	}
	sort.Strings(pkgArr)
	packSelect.Options = pkgArr
	var selectTableMap map[string]string
	// 第一级选择事件处理
	packSelect.OnChanged = func(selected string) {
		subItems, exists := tableTreeMap[alias2pkg[selected]]
		if !exists {
			tableSelect.Options = []string{}
			tableSelect.Refresh()
			tableDisplay.RemoveAll()
			tableDisplay.Refresh()
			return
		}

		var aliasArr []string
		selectTableMap = make(map[string]string)
		for k, v := range subItems {
			aliasArr = append(aliasArr, v.Alias)
			selectTableMap[v.Alias] = k
		}
		sort.Strings(aliasArr)

		tableSelect.Options = aliasArr
		tableSelect.ClearSelected()
		tableDisplay.RemoveAll()
		tableDisplay.Refresh()
	}

	// 第二级选择事件处理
	tableSelect.OnChanged = func(selected string) {
		if selected == "" || packSelect.Selected == "" {
			return
		}

		if table, exists := tableTreeMap[alias2pkg[packSelect.Selected]][selectTableMap[selected]]; exists {
			typMap := utils.AllTypMap[alias2pkg[packSelect.Selected]]
			OnSelectTable(table, typMap, tableDisplay)
		}
	}
	return container.NewHBox(packSelect, tableSelect)
}

func OnSelectTable(table *TableTree, typMap map[string]utils.Meta, tableDisplay *fyne.Container) {
	tableDisplay.RemoveAll()
	for _, child := range table.Nodes {
		AddNode(child, typMap, tableDisplay)
	}
	selectTable = table
	tableDisplay.Refresh()
}
func AddNode(node *TreeNode, typMap map[string]utils.Meta, c *fyne.Container) *fyne.Container {
	contentContainer := container.NewGridWithColumns(4)
	switch node.Typ {
	case "int":
		AddInt(node, contentContainer)
	case "string":
		AddString(node, contentContainer)
	case "bool":
		AddBool(node, contentContainer)
	case "list":
		AddList(node, typMap, contentContainer)
	case "map":
		AddMap(node, typMap, contentContainer)
	default:
		meta, ok := typMap[node.Typ]
		if !ok || meta.Typ == utils.TABLE {
			return nil
		}
		if meta.Typ == utils.STRUCT {
			AddStruct(node, typMap, contentContainer)
		} else if meta.Typ == utils.ENUM {
			AddEnum(node, meta, contentContainer)
		}
	}
	c.Add(contentContainer)
	return contentContainer
}

func AddList(node *TreeNode, typMap map[string]utils.Meta, nodeContainer *fyne.Container) {
	titleContainer := container.NewHBox()
	titleContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	contentContainer := container.NewVBox()
	var addBtn *widget.Button
	list, ok := node.Val.([]*TreeNode)
	if !ok {
		return
	}
	for i, val := range list {
		var subNodeContainer *fyne.Container
		rmvBtn := widget.NewButton("移除", func() {
			list[i].Deleted = true
			node.Val = list
			subNodeContainer.RemoveAll()
			subNodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):已移除", list[i].Alias, list[i].Name)))
			subNodeContainer.Refresh()
		})
		subNodeContainer = AddNode(val, typMap, contentContainer)
		subNodeContainer.Add(rmvBtn)
		subNodeContainer.Refresh()
	}

	addBtn = widget.NewButton("增加", func() {
		index := len(list)
		addStructVar := utils.StructVar{
			Name:  fmt.Sprintf("%s_%d", node.Name, index),
			Alias: fmt.Sprintf("%s_%d", node.Alias, index),
			Typ:   node.ValTyp,
		}
		addNode := InitTableNode(node.Level, addStructVar, typMap)
		FillNodeData(addNode, typMap, nil)
		list = append(list, addNode)
		node.Val = list
		var subNodeContainer *fyne.Container
		rmvBtn := widget.NewButton("移除", func() {
			list[index].Deleted = true
			node.Val = list
			subNodeContainer.RemoveAll()
			subNodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):已移除", list[index].Alias, list[index].Name)))
			subNodeContainer.Refresh()
		})
		subNodeContainer = AddNode(list[index], typMap, contentContainer)
		subNodeContainer.Add(rmvBtn)
		subNodeContainer.Refresh()
	})
	editBtn := AddEditBtn(node.Alias, titleContainer, contentContainer, addBtn)
	titleContainer.Add(editBtn)
	nodeContainer.Add(titleContainer)
}

func AddMap(node *TreeNode, typMap map[string]utils.Meta, nodeContainer *fyne.Container) {
	titleContainer := container.NewHBox()
	titleContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	contentContainer := container.NewVBox()
	var addBtn *widget.Button
	list, ok := node.Val.([]*TreeNode)
	if !ok {
		return
	}
	for i, val := range list {
		var subNodeContainer *fyne.Container
		rmvBtn := widget.NewButton("移除", func() {
			list[i].Deleted = true
			node.Val = list
			subNodeContainer.RemoveAll()
			subNodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):已移除", list[i].Alias, list[i].Name)))
			subNodeContainer.Add(widget.NewLabel(list[i].Key))
			subNodeContainer.Refresh()
		})
		subNodeContainer = AddNode(val, typMap, contentContainer)
		InsertKey(subNodeContainer, val, rmvBtn)
		subNodeContainer.Refresh()
	}

	addBtn = widget.NewButton("增加", func() {
		index := len(list)
		addStructVar := utils.StructVar{
			Name:  fmt.Sprintf("%s_%d", node.Name, index),
			Alias: fmt.Sprintf("%s_%d", node.Alias, index),
			Typ:   node.ValTyp,
		}
		addNode := InitTableNode(node.Level, addStructVar, typMap)
		FillNodeData(addNode, typMap, nil)
		list = append(list, addNode)
		node.Val = list
		var subNodeContainer *fyne.Container
		rmvBtn := widget.NewButton("移除", func() {
			list[index].Deleted = true
			node.Val = list
			subNodeContainer.RemoveAll()
			subNodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):已移除", list[index].Alias, list[index].Name)))
			subNodeContainer.Add(widget.NewLabel(list[index].Key))
			subNodeContainer.Refresh()
		})
		subNodeContainer = AddNode(list[index], typMap, contentContainer)
		InsertKey(subNodeContainer, list[index], rmvBtn)
		subNodeContainer.Refresh()
	})
	editBtn := AddEditBtn(node.Alias, titleContainer, contentContainer, addBtn)
	titleContainer.Add(editBtn)
	nodeContainer.Add(titleContainer)
}

func InsertKey(subNodeContainer *fyne.Container, val *TreeNode, rmvBtn *widget.Button) {
	title, content := subNodeContainer.Objects[0], subNodeContainer.Objects[1]
	subNodeContainer.RemoveAll()
	subNodeContainer.Add(title)
	input := widget.NewEntryWithData(binding.BindString(&val.Key))
	input.SetPlaceHolder("enter string")
	input.OnChanged = func(text string) {
		val.Key = text
	}
	subNodeContainer.Add(input)
	subNodeContainer.Add(content)
	subNodeContainer.Add(rmvBtn)
}

func AddEnum(node *TreeNode, meta utils.Meta, nodeContainer *fyne.Container) {
	options := make([]string, 0)
	optionsMap := make(map[string]int)
	optionsRevtMap := make(map[int]string)
	for _, v := range meta.Meta.(*utils.Enum).Vars {
		options = append(options, fmt.Sprintf("%s(%s)", v.Alias, v.Name))
		num, err := strconv.Atoi(v.Default)
		if err != nil {
			continue
		}
		optionsMap[fmt.Sprintf("%s(%s)", v.Alias, v.Name)] = num
		optionsRevtMap[num] = fmt.Sprintf("%s(%s)", v.Alias, v.Name)
	}
	enumSelect := widget.NewSelect(options, func(text string) {
		node.Val = optionsMap[text]
	})
	val, ok := node.Val.(int)
	if !ok {
		val = -1
	}
	enumSelect.PlaceHolder = "选择" + node.Typ
	selectedEnum, ok := optionsRevtMap[val]
	if val > 0 && ok {
		enumSelect.Selected = selectedEnum
	}
	nodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	nodeContainer.Add(enumSelect)
}

func AddStruct(node *TreeNode, typMap map[string]utils.Meta, nodeContainer *fyne.Container) {
	titleContainer := container.NewHBox()
	titleContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	contentContainer := container.NewVBox()
	editBtn := AddEditBtn(node.Alias, titleContainer, contentContainer, nil)
	titleContainer.Add(editBtn)
	nodeContainer.Add(titleContainer)
	for _, child := range node.Nodes {
		AddNode(child, typMap, contentContainer)
	}
}

func AddBool(node *TreeNode, nodeContainer *fyne.Container) {
	val, ok := node.Val.(bool)
	if !ok {
		val = false
	}
	isTrueStr := "布尔值"
	nodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	radio := widget.NewRadioGroup([]string{isTrueStr}, func(text string) {
		if text == isTrueStr {
			node.Val = true
		} else {
			node.Val = false
		}
	})
	if val {
		radio.Selected = isTrueStr
	}
	nodeContainer.Add(radio)
}

func AddString(node *TreeNode, nodeContainer *fyne.Container) {
	val, ok := node.Val.(string)
	if !ok {
		val = ""
	}
	nodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	input := widget.NewEntryWithData(binding.BindString(&val))
	input.SetPlaceHolder("enter string")
	input.OnChanged = func(text string) {
		node.Val = text
	}
	nodeContainer.Add(input)
}

func AddInt(node *TreeNode, nodeContainer *fyne.Container) {
	val, ok := node.Val.(int)
	if !ok {
		val = 0
	}
	nodeContainer.Add(widget.NewLabel(fmt.Sprintf("%s(%s):", node.Alias, node.Name)))
	input := widget.NewEntryWithData(binding.IntToString(binding.BindInt(&val)))
	input.SetPlaceHolder("enter int")
	input.OnChanged = func(text string) {
		num, err := strconv.Atoi(text)
		if err != nil {
			return
		}
		node.Val = num
	}
	nodeContainer.Add(input)
}

func AddEditBtn(title string, titleContainer, content *fyne.Container, addBtn *widget.Button) *widget.Button {
	var editBtn *widget.Button

	editBtn = widget.NewButton("编辑详情", func() {
		editBtn.Disable()
		editBtn.Refresh()
		w := container.NewInnerWindow(title, container.NewBorder(nil, addBtn, nil, nil, container.NewVScroll(content)))
		if addBtn == nil {
			w = container.NewInnerWindow(title, container.NewBorder(nil, nil, nil, nil, container.NewVScroll(content)))
		}
		w.Resize(fyne.NewSize(Width/2, High/2))
		w.Move(fyne.NewPos(Width/4, High/4))
		w.CloseIntercept = func() {
			w.Close()
			editBtn.Enable()
			editBtn.Refresh()
		}
		wCtrl.Add(w)
	})
	return editBtn
}
