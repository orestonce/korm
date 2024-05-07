package korm

import (
	"bytes"
	"strconv"
)

func (this *GoSourceWriter) AddFunc_View_Select(info StructType) {
	this.buf.WriteString(`type ` + info.GetOrmSubType_SelectName() + ` struct {
	supper *` + ormAllName + `
	bufWhere bytes.Buffer
	argsWhere []interface{}
	orderBy []string
	limit   int
	offset  int
	linkOpList []string
	isLinkBegin bool
	joinNode    *` + this.getLeftJoinNodeName() + `
`)
	begin, ok := this.structMap[info.ViewBeginD]
	if ok == false || begin.IsView {
		panic(`GoSourceWriter.AddFunc_View_Select ` + strconv.Quote(info.Name) + "." + info.ViewBeginD)
	}
	this.buf.WriteString(`query *` + begin.GetOrmSubType_SelectName() + "// ViewBeginD\n")
	info.View_HasCallMap = map[string]string{} // path -> varName

	for idx, one := range info.FieldList {
		if len(one.ViewJoinPath) <= 1 { // 自己这一级的
			continue
		}
		call := one.ViewGetJoinObj_CallDeclear()
		if info.View_HasCallMap[call] != "" {
			continue
		}
		lastTypeName := one.ViewGetJoinObjPath_LastBeLeftJoin(begin, this)

		varName := `node_` + strconv.Itoa(idx)
		info.View_HasCallMap[call] = varName
		info.View_VarNameCallList = append(info.View_VarNameCallList, [2]string{
			varName, call,
		})
		this.buf.WriteString(varName + " *" + lastTypeName + "\n")
	}
	this.buf.WriteString(`}

func (this *` + info.GetOrmTypeName() + `) Select() *` + info.GetOrmSubType_SelectName() + ` {
	query := this.supper.` + info.ViewBeginD + `().Select()
	one := &` + info.GetOrmSubType_SelectName() + `{
		supper: this.supper, linkOpList: []string{"AND"}, isLinkBegin: true,
		query:  query,
`)
	for _, one := range info.View_VarNameCallList {
		varName := one[0]
		call := one[1]
		this.buf.WriteString(varName + ": " + call + ",\n")
	}
	this.buf.WriteString(`
	}
	one.joinNode = query.joinNode
	one.joinNode.Root = &query.rootInfo
	return one
} 
`)
	tName := info.GetOrmSubType_SelectName()
	this.AddWhereAllFieldL1(info, tName, false, func(fieldName string, fnName string) string {
		nodeName, fieldName2 := info.View_getQueryNode_ByFieldName(fieldName)

		buf := bytes.NewBuffer(nil)
		buf.WriteString("this.supper.bufWhere.WriteString(")
		if fnName != "" {
			buf.WriteString(`"` + fnName + `(" + `)
		}
		buf.WriteString(nodeName + ".supper.joinNode.TableName+`.`+")
		buf.WriteString(`"` + quoteForSql(fieldName2))
		if fnName != "" {
			buf.WriteString(")")
		}
		buf.WriteString(` ")` + "\n")
		return buf.String()
	})
	this.AddFuncOrderByL1(tName, info, false, func(fn string) (tn string, fn2 string) {
		nodeName, fieldName2 := info.View_getQueryNode_ByFieldName(fn)
		return nodeName + ".joinNode.TableName", fieldName2
	})
	this.setLimitOffset(tName)
	this.buildFn_Count_Exist(tName, info.ViewBeginD)
	this.buildFn_ResultOne(tName, info.Name)
	this.buildFn_ResultOne2(tName, info)
	this.buildFn_ResultList(tName, info)
	this.buildFn_ResultMap(tName, info)
	this.buildFn_ResultTotalMatch(tName, info)
}

func (this *GoSourceWriter) setLimitOffset(tName string) {
	this.buf.WriteString(`
func (this *` + tName + `) LimitOffset(limit int, offset int) *` + tName + ` {
	this.limit = limit
	this.offset = offset
	return this
}

// pageSize: [1, n)
// pageNo:   [1,n) 
func (this *` + tName + `) SetPageLimit(pageSize int, pageNo int) *` + tName + `{
	if pageSize <= 0 || pageNo <= 0 {
		panic("` + tName + ` SetPageLimit error param")
	}
	this.limit = pageSize
	this.offset = pageSize * (pageNo -1)
	return this
}`)
}

func (this StructFieldType) ViewGetJoinObjPath_LastBeLeftJoin(begin StructType, w *GoSourceWriter) string {
	pt := this.ViewGetJoinObjPath()
	v := &begin
	for _, ot := range pt {
		if v == nil || v.IsView {
			panic(`ViewGetJoinObjPath_LastBeLeftJoin ` + strconv.Quote(ot))
		}
		var foundF *StructFieldType
		for _, f := range v.FieldList {
			if f.FiledName == ot {
				foundF = &f
				break
			}
		}
		if foundF == nil {
			v = nil
			continue
		}
		f, ok := w.structMap[foundF.TypeName]
		if ok == false || f.IsView {
			v = nil
			continue
		}
		v = &f
	}
	structName := pt[len(pt)-1]
	if v == nil {
		panic("ViewGetJoinObjPath_LastBeLeftJoin " + strconv.Quote(structName))
	}
	return v.GetLeftJoinName()
}

func (this StructFieldType) ViewGetJoinObjPath() []string {
	return this.ViewJoinPath[:len(this.ViewJoinPath)-1]
}

func (this StructFieldType) ViewGetJoinObj_CallDeclear() string {
	buf := bytes.NewBuffer(nil)
	buf.WriteString("query")
	for _, str := range this.ViewGetJoinObjPath() {
		buf.WriteString(`.LeftJoin_` + str + "()")
	}
	return buf.String()
}
