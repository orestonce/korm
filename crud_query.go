package korm

import (
	"bytes"
	"strconv"
)

func (this *GoSourceWriter) AddFunc_ORM_Query(info StructType) {
	this.buf.WriteString(`// Select ` + info.Name + "\n")
	this.AddImportPath("strings")
	this.AddImportPath("bytes")
	tName := info.GetOrmSubType_SelectName()
	this.buf.WriteString(`type ` + tName + ` struct {
	supper *` + ormAllName + `
	bufWhere bytes.Buffer
	argsWhere []interface{}
	orderBy []string
	limit   int
	offset  int
	linkOpList []string
	isLinkBegin bool
	joinNode    *` + this.getLeftJoinNodeName() + `
	rootInfo    ` + this.getLeftJoinRootInfoName() + `
}

func (this *` + info.GetOrmTypeName() + `) Select() *` + tName + `{
	one := &` + info.GetOrmSubType_SelectName() + `{supper: this.supper, linkOpList: []string{"AND"}, isLinkBegin: true}
	one.joinNode = &` + this.getLeftJoinNodeName() + `{TableName: "_0"}	
	one.joinNode.Root = &one.rootInfo
	one.rootInfo.TableNameIdx = 1
	return one
}
`)
	this.AddFuncOrderBy(tName, info, false)
	this.AddImportPath("strconv")
	this.setLimitOffset(tName)
	this.buildFn_Count_Exist(tName, info.Name)
	this.buildFn_ResultOne(tName, info.Name)
	this.buildFn_ResultOne2(tName, info)
	this.buildFn_ResultList(tName, info)
	this.buildFn_ResultMap(tName, info)
	this.buildFn_ResultTotalMatch(tName, info)
	this.AddImportPath("context")
	for _, one := range info.FieldList {
		if one.IsStarExpr == false {
			continue
		}
		otherStruct := this.structMap[one.TypeName]

		ljName := otherStruct.GetLeftJoinName()
		this.buf.WriteString(`func (this *` + tName + `) LeftJoin_` + one.FiledName + `() *` + ljName + `{
	node := this.joinNode.AddLeftJoin("` + one.TypeName + `", ` + strconv.Quote(one.FiledName) + `, "` + one.LjField_This + `", "` + one.LjFiled_Other + `")
	return &` + ljName + `{
		joinNode:     node,
		bufWhere:     &this.bufWhere,
		argsWhereP:   &this.argsWhere,
		isLinkBeginP: &this.isLinkBegin,
		linkOpListP:  &this.linkOpList,
		orderByP:     &this.orderBy,
}
}
`)
		this.defineLeftJoinType(ljName, otherStruct)
	}
}

func (info *StructType) BuildQueryString(this *GoSourceWriter, ret string) string {
	var buf bytes.Buffer
	info.BuildQueryStringL1(&buf, "")
	buf.WriteString(`
	var result *sql.Rows
	` + this.writeErrDecl_IfMust() + `
` + createCodeQuery(createCodeQuery_Req{
		arg0:   "buf2.String(), this.argsWhere...",
		isExec: false,
		ret0:   "result, err",
	}) + this.writeCheckErr(ret) + `
	defer result.Close()
	`)
	return buf.String()
}

func (info *StructType) BuildQueryStringOnConn() string {
	var buf bytes.Buffer
	info.BuildQueryStringL1(&buf, "SQL_CALC_FOUND_ROWS ")
	buf.WriteString(`var conn *sql.Conn
	var result *sql.Rows
	if this.supper.db != nil {
		var err error
		conn, err = this.supper.db.Conn(context.Background())
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		result, err = conn.QueryContext(context.Background(), buf2.String(), this.argsWhere...)
		if err != nil {
			panic(err)
		}
	} else {
		var err error
		result, err = this.supper.tx.Query(buf2.String(), this.argsWhere...)
		if err != nil {
			panic(err)
		}
	}
`)
	return buf.String()
}

func (info *StructType) BuildQueryStringL1(buf *bytes.Buffer, selectPrefixMysql string) {
	buf.WriteString(info.isD_CallFillNameList() + `
	var buf2 bytes.Buffer
	buf2.WriteString("SELECT ")
`)
	if selectPrefixMysql != "" {
		buf.WriteString(`if this.supper.mode == korm.InitModeMysql {
	buf2.WriteString("` + selectPrefixMysql + ` ")
}
`)
	}
	if info.IsView {
		bufIn := bytes.NewBuffer(nil)
		for idx, one := range info.FieldList {
			if idx > 0 {
				bufIn.WriteString(` + "," + `)
			}
			node, fn := info.View_getQueryNode_ByFieldName(one.FiledName)
			bufIn.WriteString(node + `.joinNode.TableName + "." + ` + "\"`" + fn + "`\" ")
		}
		buf.WriteString("buf2.WriteString(" + bufIn.String() + ")\n")
	} else {
		buf.WriteString(`this.joinNode.FillSelect(&buf2, true)` + "\n")
	}
	infoDotName := info.Name
	if info.IsView {
		infoDotName = info.ViewBeginD
	}
	buf.WriteString(`
	buf2.WriteString(" FROM ` + quoteForSql(infoDotName) + ` "+this.joinNode.TableName+" ")
	buf2.WriteString(this.` + func() string {
		if info.IsView {
			return "query."
		}
		return ""
	}() + `rootInfo.BufLeftJoin.String())
	` + gAddWhereToBuf2 + `
	if len(this.orderBy) > 0 {
		buf2.WriteString("ORDER BY " + strings.Join(this.orderBy, ",") + " ")
	}
	if this.limit != 0 {
		buf2.WriteString("LIMIT " + strconv.Itoa(this.limit) + " ")
	}
	if this.offset != 0 {
		buf2.WriteString("OFFSET " + strconv.Itoa(this.offset) + " ")
	}
`)
}

type AddWhereAllField_Req struct {
	isLinkBegin string
	linkOpList  string
	argsWhere   string
}

func (this *GoSourceWriter) AddWhereAllField(info StructType, name string, isLeftJoinPart bool, needTableName bool) {
	this.AddWhereAllFieldL1(info, name, isLeftJoinPart, func(fieldName string) string {
		buf := bytes.NewBuffer(nil)
		buf.WriteString("this.bufWhere.WriteString(")
		if needTableName {
			buf.WriteString("this.joinNode.TableName+`.`+")
		}
		buf.WriteString(`"` + quoteForSql(fieldName) + ` ")` + "\n")
		return buf.String()
	})
}

func (this *GoSourceWriter) AddWhereAllFieldL1(info StructType, name string, isLeftJoinPart bool, fnWField func(fieldName string) string) {
	req := AddWhereAllField_Req{
		isLinkBegin: "this.isLinkBegin",
		linkOpList:  "this.linkOpList",
		argsWhere:   "this.supper.argsWhere",
	}
	if isLeftJoinPart {
		req.isLinkBegin = `(*this.isLinkBeginP)`
		req.linkOpList = `(*this.linkOpListP)`
		req.argsWhere = `(*this.supper.argsWhereP)`
	}
	for _, f := range info.FieldList {
		this.FiledOp_AddWhere(name, f, req, fnWField)
	}
	this.ChangeLinkOpBegin(name, "AND", req)
	this.ChangeLinkOpBegin(name, "OR", req)
	this.ChangeLinkOpEnd(name, req)
}

func (this *GoSourceWriter) FiledOp_AddWhere(name string, f StructFieldType, req AddWhereAllField_Req, fnWField func(fieldName string) string) {
	ftNameLink := prefix + `Where_` + name
	ftName := ftNameLink + `_` + f.FiledName

	//	callAndTableName := func() string {
	//		if needTableName == false {
	//			return ""
	//		}
	//		return `this.bufWhere.WriteString(` + strconv.Quote("`") + "+this.joinNode.TableName+" + strconv.Quote("`.") + `)
	//`
	//	}
	//` + callAndTableName() + `this.bufWhere.WriteString("` + quoteForSql(f.FiledName) + ` ")
	writeTypeAndCond := func() {
		this.buf.WriteString(`type ` + ftName + ` struct{
supper *` + name + `
}
func (this *` + name + `) Where` + `_` + f.FiledName + `() *` + ftName + `{
	if this.bufWhere.Len() > 0 {
		if ` + req.isLinkBegin + ` == false {
			this.bufWhere.WriteString(` + req.linkOpList + `[0] + " ")
		}
	} else {
		this.bufWhere.WriteString("WHERE ")
	}
	` + fnWField(f.FiledName) + `
	` + req.isLinkBegin + ` = false
	return &` + ftName + `{supper: this}
}
`)
	}
	declStr := ""
	argsName := f.FiledName
	addFn := func(op string, sqlStr string) {
		this.buf.WriteString(`func (this *` + ftName + `)` + op + `(` + f.FiledName + ` ` + f.TypeName + `) *` + name + ` {
	` + declStr + `this.supper.bufWhere.WriteString("` + sqlStr + ` ")
	` + req.argsWhere + ` = append(` + req.argsWhere + `, ` + argsName + `)
	return this.supper
}
`)
	}
	switch f.TypeName {
	default:
		if f.TypeName == "string" || f.IsIntOrUintType() {
			writeTypeAndCond()
			addFn("Equal", "=?")
			addFn("NotEqual", "!=?")
			addFn("Greater", ">?")
			addFn("GreaterOrEqual", ">=?")
			addFn("Less", "<?")
			addFn("LessOrEqual", "<=?")
			this.buf.WriteString(`func (this *` + ftName + `) In(vList []` + f.TypeName + `) *` + name + ` {
if len(vList) == 0 {
	this.supper.bufWhere.WriteString("= '' AND 0 ")	// 什么都不存在, 直接返回
	return this.supper
}
this.supper.bufWhere.WriteString("IN (")
for idx, v := range vList {
	if idx > 0 {
		this.supper.bufWhere.WriteString(", ")
	}
	this.supper.bufWhere.WriteString("?")
	` + req.argsWhere + ` = append(` + req.argsWhere + `, v)
}
this.supper.bufWhere.WriteString(") ")
return this.supper
}
`)
		}
	case "bool":
		writeTypeAndCond()
		declStr = `v` + f.FiledName + ` := strconv.FormatBool(` + f.FiledName + `)` + "\n"
		argsName = "v" + f.FiledName
		addFn("Equal", "=?")
	case "time.Time":
		writeTypeAndCond()
		declStr = `v` + f.FiledName + ` := ` + f.FiledName + `.UTC().Format(time.RFC3339Nano)` + "\n"
		argsName = "v" + f.FiledName
		addFn("Equal", "=?")
		addFn("NotEqual", "!=?")
		addFn("GreaterOrEqual", ">=?")
		addFn("Less", "<?")
		addFn("LessOrEqual", "<=?")
	}
}

func (t StructType) GetOrmSubType_SelectName() string {
	return prefix + t.Name + "_SelectObj"
}

func (this *GoSourceWriter) writeStr_IfMustValue_Equal(v bool, s string) string {
	if v == this.req.GenMustFn {
		return s
	}
	return ""
}
