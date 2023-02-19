package korm

import "strconv"

func (this *GoSourceWriter) AddFuncOrderBy(tName string, info StructType, isPointer bool) {
	v := "this.supper.orderBy"
	if isPointer {
		v = `(*this.supper.orderByP)`
	}
	subName := tName + `_OrderByObj`
	if this.orderByNameMap[subName] == false {
		this.orderByNameMap[subName] = true
		this.buf.WriteString(`type ` + subName + ` struct {
fieldName string
supper *` + tName + `
}
`)
		for _, order := range []string{"ASC", "DESC"} {
			this.buf.WriteString(`func (this *` + subName + `) ` + order + `() *` + tName + ` {
	` + v + ` = append(` + v + `, this.supper.joinNode.tableName + ".` + "`\" + this.fieldName + \"" + "`" + ` ` + order + ` ")
	return this.supper
}
`)
		}
	}
	for _, f := range info.FieldList {
		switch f.TypeName {
		case "string", "int", "int64", "time.Time":
			this.buf.WriteString(`func (this *` + tName + `) OrderBy_` + f.FiledName + `() *` + subName + ` {
	return &` + subName + `{
		fieldName: ` + strconv.Quote(f.FiledName) + `,
		supper: this,
	}
}
`)
		}
	}
}
