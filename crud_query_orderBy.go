package korm

import "strconv"

func (this *GoSourceWriter) AddFuncOrderBy(tName string, info StructType, isPointer bool) {
	this.AddFuncOrderByL1(tName, info, isPointer, func(fn string) (tn string, fn2 string) {
		return "this.joinNode.TableName", fn
	})
}
func (this *GoSourceWriter) AddFuncOrderByL1(tName string, info StructType, isPointer bool, getTableNameAndFieldNameFn func(fn string) (tn string, fn2 string)) {
	v := "this.supper.orderBy"
	if isPointer {
		v = `(*this.supper.orderByP)`
	}
	subName := tName + `_OrderByObj`
	if this.orderByNameMap[subName] == false {
		this.orderByNameMap[subName] = true
		this.buf.WriteString(`type ` + subName + ` struct {
tableName string
fieldName string
supper *` + tName + `
}
`)
		for _, order := range []string{"ASC", "DESC"} {
			//this.supper.joinNode.TableName + ".` + "`\" + this.fieldName + \"" + "`" + `
			this.buf.WriteString(`func (this *` + subName + `) ` + order + `() *` + tName + ` {
	` + v + ` = append(` + v + `, this.tableName + ".` + "`\" + this.fieldName + \"" + "`" + ` ` + order + ` ")
	return this.supper
}
`)
		}
	}
	for _, f := range info.FieldList {
		switch f.TypeName {
		case "string", "int", "int64", "time.Time":
			tableName, fieldName := getTableNameAndFieldNameFn(f.FiledName)
			this.buf.WriteString(`func (this *` + tName + `) OrderBy_` + f.FiledName + `() *` + subName + ` {
	return &` + subName + `{
		fieldName: ` + strconv.Quote(fieldName) + `,
		tableName: ` + tableName + `,
		supper: this,
	}
}
`)
		}
	}
}
