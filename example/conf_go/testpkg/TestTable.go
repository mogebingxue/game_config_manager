// Code generated by gen_cfg_go. DO NOT EDIT.

// 测试包
package testpkg

import "github.com/mogebingxue/game_config_manager"

// 测试表
type TestTable struct {
	TestInt    int                  // 测试整型
	TestString string               // 测试字符串
	TestBool   bool                 // 测试布尔值
	TestEnum   TEST_ENUM            // 测试枚举
	TestList   []TEST_ENUM          // 测试列表
	TestMap    map[string]TEST_ENUM // 测试哈希表
	TestStruct TestStruct           // 测试结构
}

var testTable *TestTable
var reloadTestTable *TestTable

func (cfg *TestTable) GetFileName() string {
	return "testpkg/TestTable.json"
}

func (cfg *TestTable) GetResult() interface{} {
	return testTable
}

func (cfg *TestTable) GetReloadResult(alloc bool) interface{} {
	if alloc || reloadTestTable == nil {
		reloadTestTable = new(TestTable)
	}
	return reloadTestTable
}

func (cfg *TestTable) OnReloadFinished() {
	testTable = reloadTestTable
}

func GetTestTable() *TestTable {
	if testTable == nil {
		testTable = &TestTable{}
		config.GetConfigManager().LoadFile(testTable)
	}
	if config.GetConfigManager().IsDirty(testTable.GetFileName()) {
		config.GetConfigManager().ReloadFile(testTable)
	}
	return testTable
}
