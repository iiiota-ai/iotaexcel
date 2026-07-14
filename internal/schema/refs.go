// Package schema 的 refs.go 负责 ref<T> 的跨表引用校验。
//
// schema 解析阶段只确认 ref<T> 的语法和目标名是否合法；开启 --check-ref 后，
// app 层会先收集所有 sheet 的 key，再调用这里检查每个引用值是否真实存在。
package schema

import (
	"fmt"

	"iotaexcel/internal/model"
)

// RefIndex 保存所有已解析 sheet 的唯一 key 集合。
// 第一层 key 是 sheet 名，第二层 key 是该 sheet 中的行 key 文本。
type RefIndex struct {
	keys map[string]map[string]bool
}

// NewRefIndex 创建空引用索引。
func NewRefIndex() *RefIndex {
	return &RefIndex{keys: map[string]map[string]bool{}}
}

// Add 把一个 sheet 的全部唯一 key 加入索引。
// name 通常使用 sheet.Name，必须和 ref<T> 中的 T 大小写一致。
func (r *RefIndex) Add(name string, sheet model.Sheet) {
	if r.keys[name] == nil {
		r.keys[name] = map[string]bool{}
	}
	for _, row := range sheet.Rows {
		r.keys[name][row.Key] = true
	}
}

// CheckRefs 校验 workbook 内所有 ref<T> 字段。
// 空引用或使用默认值的单元格按 null 处理，不报错；非空引用必须命中目标 sheet 的 key。
func CheckRefs(wb *model.Workbook, refs *RefIndex) {
	for si := range wb.Sheets {
		sheet := &wb.Sheets[si]
		for _, field := range sheet.Fields {
			if field.Type.Kind != model.TypeRef {
				continue
			}
			target := field.Type.Inner
			targetKeys := refs.keys[target]
			if targetKeys == nil {
				sheet.RefErrors = append(sheet.RefErrors, fmt.Sprintf("field %s references missing table %s", field.Name, target))
				continue
			}
			for _, row := range sheet.Rows {
				cell := row.Values[field.Name]
				if cell.Default {
					continue
				}
				key, ok := cell.Value.(string)
				if !ok || key == "" {
					continue
				}
				if !targetKeys[key] {
					sheet.RefErrors = append(sheet.RefErrors, fmt.Sprintf("row %d field %s references missing key %s.%s", row.Index, field.Name, target, key))
				}
			}
		}
	}
}
