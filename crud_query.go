package korm

import (
	"bytes"
	"strconv"
	"strings"
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
	one.joinNode = &` + strings.ToLower(prefix) + `leftJoinNode{tableName: "_0"}	
	one.joinNode.root = &one.rootInfo
	one.rootInfo.tableNameIdx = 1
	return one
}
`)
	this.AddFuncOrderBy(tName, info, false)
	this.AddImportPath("strconv")
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
}

func (this * ` + tName + `) ` + this.writeFnDecl("Run_Count", "cnt int64"))
	this.buf.WriteString(`
	var buf2 bytes.Buffer
	buf2.WriteString("SELECT COUNT(1) FROM ` + quoteForSql(info.Name) + ` " + this.joinNode.tableName + " ")
	buf2.WriteString(this.rootInfo.bufLeftJoin.String())
	buf2.WriteString(this.bufWhere.String())

	result, err := this.supper.ExecRawQuery(buf2.String(), this.argsWhere...)
` + this.writeCheckErr("0, ") + `
	defer result.Close()
	if result.Next() == false {
`)
	if this.req.GenMustFn {
		this.buf.WriteString(`panic("` + tName + ` not found.")`)
	} else {
		this.buf.WriteString(`return 0, errors.New("` + tName + ` not found.")`)
	}
	this.buf.WriteString(`
	}
	err = result.Scan(&cnt)
	` + this.writeCheckErr("0, ") + `
	return cnt` + this.writeStr_IfMustValue_Equal(false, ", nil") + `
}


func (this *` + tName + `) ` + this.writeFnDecl("Run_Exist", "exist bool") + `
	var buf2 bytes.Buffer
	buf2.WriteString("SELECT COUNT(1) FROM ` + quoteForSql(info.Name) + ` " + this.joinNode.tableName + " ")
	buf2.WriteString(this.rootInfo.bufLeftJoin.String())
	buf2.WriteString(this.bufWhere.String())
	buf2.WriteString(" LIMIT 1 ")
	

	result, err := this.supper.ExecRawQuery(buf2.String(), this.argsWhere...)
` + this.writeCheckErr("false, ") + `
	defer result.Close()
	if result.Next() == false {
		panic("` + tName + ` not found.")
	}
	var cnt int64
	err = result.Scan(&cnt)
` + this.writeCheckErr("false, ") + `

return cnt > 0` + this.writeStr_IfMustValue_Equal(false, ",nil") + `
}
`)
	this.buf.WriteString(`
func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultOne2", `info `+info.Name+`, ok bool`) + `
	this.limit = 1
	` + info.BuildQueryString(this, `info, ok, `))
	if this.req.GenMustFn {
		this.buf.WriteString(`if result.Next() == false {
		return info, false
	}
`)
	} else {
		this.buf.WriteString(`if result.Next() == false {
		return info, false, nil
	}
`)
	}
	this.buf.WriteString(info.getScanAndParseFnCode(this, "info, false,") + `
	return info, true`)
	if this.req.GenMustFn == false {
		this.buf.WriteString(", nil")
	}
	this.buf.WriteString(`
}

func (this *` + tName + `)` + this.writeFnDecl("Run_ResultOne", `info `+info.Name))
	if this.req.GenMustFn {
		this.buf.WriteString(`info, _ = this.MustRun_ResultOne2()
	return info`)
	} else {
		this.buf.WriteString(`info, _, err = this.Run_ResultOne2()
	return info, err`)
	}
	this.buf.WriteString(`
}
func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultList", "list []"+info.Name) + `
	` + info.BuildQueryString(this, `list,`) + `
	for result.Next() {
		var info ` + info.Name + `
		` + info.getFNName_fillSelectFieldNameList() + `(this.joinNode)
		` + info.getScanAndParseFnCode(this, "list,") + `
		list = append(list, info)
	}
	return list` + this.writeStr_IfMustValue_Equal(false, `,nil`) + `
}`)
	declearMapType := func(startIdx int) string {
		buf := bytes.NewBuffer(nil)
		for _, f := range info.FieldList[startIdx:] {
			if f.IsPrimaryKeyPart == false {
				break
			}
			buf.WriteString("map[" + f.TypeName + "]")
		}
		buf.WriteString(info.Name)
		return buf.String()
	}
	this.buf.WriteString(`
func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultMap", `m `+declearMapType(0)) + `
	` + info.BuildQueryString(this, `m,`) + `
	for result.Next() {
		var info ` + info.Name + `
		` + info.getFNName_fillSelectFieldNameList() + `(this.joinNode)
		` + info.getScanAndParseFnCode(this, "nil,") + `
		` + func() string {
		buf := bytes.NewBuffer(nil)
		var lastVarName = "m"
		for i, f := range info.FieldList {
			if f.IsPrimaryKeyPart == false {
				break
			}
			buf.WriteString(`if ` + lastVarName + ` == nil {
` + lastVarName + ` = ` + declearMapType(i) + `{}
}
`)
			tmpVarName := lastVarName + `[info.` + f.FiledName + "]"
			lastVarName = tmpVarName
		}
		buf.WriteString(lastVarName + " = info\n")
		return buf.String()
	}() + `
	}
	return m` + this.writeStr_IfMustValue_Equal(false, `,nil`) + `
}
`)
	this.buf.WriteString(`func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultListWithTotalMatch", `list []`+info.Name+`, totalMatch int64`) + `
	` + this.writeStr_IfMustValue_Equal(true, `var err error`) + `	
	` + info.BuildQueryStringOnConn() + `
	defer result.Close()
	for result.Next() {
		var info ` + info.Name + `
		` + info.getFNName_fillSelectFieldNameList() + `(this.joinNode)
		` + info.getScanAndParseFnCode(this, "list, totalMatch,") + `
		list = append(list, info)
	}
	result.Close()
	if this.limit > 0 {
		nextQuery := ""
		if this.supper.mode == korm.InitModeMysql {
			nextQuery = "select FOUND_ROWS()"
		} else if this.supper.mode == korm.InitModeSqlite {
			buf2.Reset()
			buf2.WriteString("SELECT COUNT(1) ")
			buf2.WriteString("FROM ` + quoteForSql(info.Name) + ` " + this.joinNode.tableName + " ")
			buf2.WriteString(this.rootInfo.bufLeftJoin.String())
			buf2.WriteString(this.bufWhere.String())
			nextQuery = buf2.String()
		} else {
			panic("not support")
		}
		var result2 *sql.Rows
		if conn != nil {
			result2, err = conn.QueryContext(context.Background(), nextQuery)
		} else {
			result2, err = this.supper.tx.Query(nextQuery)
		}
		` + this.writeCheckErr("nil, 0,") + `
		defer result2.Close()
	
		if result2.Next() == false {
			panic("MustRun_ResultListWithPageInfo ")
		}
		err = result2.Scan(&totalMatch)
		` + this.writeCheckErr("nil, 0,") + `
	}
	return list, totalMatch` + this.writeStr_IfMustValue_Equal(false, ",nil") + `
}

`)
	this.AddImportPath("context")
	for _, one := range info.FieldList {
		if one.IsStarExpr == false {
			continue
		}
		otherStruct := this.structMap[one.TypeName]

		ljName := otherStruct.GetLeftJoinName()
		this.buf.WriteString(`func (this *` + tName + `) LeftJoin_` + one.FiledName + `() *` + ljName + `{
	node := this.joinNode.addLeftJoin("` + one.TypeName + `", ` + strconv.Quote(one.FiledName) + `, "` + one.LjField_This + `", "` + one.LjFiled_Other + `")
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
	buf.WriteString(info.getFNName_fillSelectFieldNameList() + `(this.joinNode)
	var buf2 bytes.Buffer
	buf2.WriteString("SELECT ")
`)
	if selectPrefixMysql != "" {
		buf.WriteString(`if this.supper.mode == korm.InitModeMysql {
	buf2.WriteString("` + selectPrefixMysql + ` ")
}
`)
	}
	buf.WriteString(`this.joinNode.fillSelect(&buf2, true)
	buf2.WriteString(" ")
	buf2.WriteString("FROM ` + quoteForSql(info.Name) + ` "+this.joinNode.tableName+" ")
	buf2.WriteString(this.rootInfo.bufLeftJoin.String())
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
	isLinkBegin    string
	linkOpList     string
	argsWhere      string
	isLeftJoinPart bool
	needTableName  bool
}

func (this *GoSourceWriter) AddWhereAllField(info StructType, name string, isLeftJoinPart bool, needTableName bool) {
	req := AddWhereAllField_Req{
		isLinkBegin:    "this.isLinkBegin",
		linkOpList:     "this.linkOpList",
		argsWhere:      "this.supper.argsWhere",
		isLeftJoinPart: isLeftJoinPart,
		needTableName:  needTableName,
	}
	if isLeftJoinPart {
		req.isLinkBegin = `*(this.isLinkBeginP)`
		req.linkOpList = `(*this.linkOpListP)`
		req.argsWhere = `*(this.supper.argsWhereP)`
	}
	for _, f := range info.FieldList {
		this.FiledOp_AddWhere(name, f, req)
	}
	this.ChangeLinkOpBegin(name, "AND", req)
	this.ChangeLinkOpBegin(name, "OR", req)
	this.ChangeLinkOpEnd(name, req)
}

func (this *GoSourceWriter) FiledOp_AddWhere(name string, f StructFieldType, req AddWhereAllField_Req) {
	ftNameLink := prefix + `Where_` + name
	ftName := ftNameLink + `_` + f.FiledName

	callAndTableName := func() string {
		if req.needTableName == false {
			return ""
		}
		return `this.bufWhere.WriteString(` + strconv.Quote("`") + "+this.joinNode.tableName+" + strconv.Quote("`.") + `)
`
	}
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
	` + callAndTableName() + `this.bufWhere.WriteString("` + quoteForSql(f.FiledName) + ` ")
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
