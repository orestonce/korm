package korm

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type MustCreateCode_Req struct {
	ModelPkgDir      string
	ModelPkgFullPath string
	ModelNameList    []string
	OutputFileName   string
	ImportExtList    []string
	GenMustFn        bool
}

const prefix = `KORM_`

func MustCreateCode(req MustCreateCode_Req) {
	os.Remove(req.OutputFileName)

	source := &GoSourceWriter{
		req:             req,
		importMap:       map[string]bool{},
		leftJoinTypeMap: map[string]struct{}{},
		orderByNameMap:  map[string]bool{},
	}
	for _, one := range req.ImportExtList {
		source.AddImportPath(one)
	}
	source.MustParsePkg(req)
	source.AddFunc_DbType()
	source.AddFunc_InitTable()
	source.writeScanInfoCode()
	for _, name := range req.ModelNameList {
		info := source.structMap[name]
		source.AddDeclear_ORM_Type(info)
		if info.IsView {
			source.AddFunc_View_Select(info)
			continue
		}
		source.AddFunc_fillSelectFieldNameList(info)
		source.AddFunc_ORM_Create(info)

		source.AddFunc_ORM_Query(info)
		source.AddWhereAllField(info, info.GetOrmSubType_SelectName(), false, true)

		source.AddFunc_ORM_Update(info)
		source.AddWhereAllField(info, info.GetOrmSubType_UpdateName(), false, false)

		source.AddFunc_ORM_Delete(info)
		source.AddWhereAllField(info, info.GetOrmSubType_Delete(), false, false)
	}
	source.FormatWriteSrc(req.OutputFileName)
}

type StructType struct {
	Name                 string
	FieldList            []StructFieldType
	IsView               bool
	ViewBeginD           string
	View_VarNameCallList [][2]string
	View_HasCallMap      map[string]string
}

func (info StructType) View_getQueryNode_ByFieldName(fn string) (nodeName string, fieldName2 string) {
	for _, one := range info.FieldList {
		if fn != one.FiledName {
			continue
		}
		if len(one.ViewJoinPath) == 0 { // 自己这一级的, 未改名
			return "this.query", one.FiledName
		} else if len(one.ViewJoinPath) == 1 { // 自己这一级的, 已改名
			return "this.query", one.ViewJoinPath[0]
		}
		call := one.ViewGetJoinObj_CallDeclear()
		varName := info.View_HasCallMap[call]
		return "this." + varName, one.ViewJoinPath[len(one.ViewJoinPath)-1]
	}
	panic("getQueryNode_ByFieldName " + fn)
}

func (t StructType) GetOrmTypeName() string {
	return prefix + t.Name
}

func (t StructType) GetDotName() string {
	if t.IsView {
		return t.ViewBeginD
	}
	return t.Name
}

func (info StructType) isD_CallFillNameList() string {
	if info.IsView {
		return ""
	}
	return info.getFNName_fillSelectFieldNameList() + `(this.joinNode)
`
}

func quoteForSql(v string) string {
	return "`" + v + "`"
}

type StructFieldType struct {
	FiledName        string
	TypeName         string
	IsStarExpr       bool
	IsPrimaryKeyPart bool
	LjField_This     string // left join
	LjFiled_Other    string // left join
	IndexList        [][]string
	ViewJoinPath     []string
}

func (this StructFieldType) IsStringOrIntUintType() bool {
	return this.TypeName == "string" || this.IsIntOrUintType()
}

func (this StructFieldType) IsBuildinTimeType() bool {
	return this.TypeName == "time.Time"
}

func (t StructFieldType) IsIntType() bool {
	for _, vs := range []string{supportTypeInt, supportTypeInt8, supportTypeInt16, supportTypeInt32, supportTypeInt64} {
		if vs == t.TypeName {
			return true
		}
	}
	return false
}

func (t StructFieldType) IsUintType() bool {
	for _, vs := range []string{supportTypeUint, supportTypeUint8, supportTypeUint16, supportTypeUint32, supportTypeUint64} {
		if vs == t.TypeName {
			return true
		}
	}
	return false
}

func (t StructFieldType) IsIntOrUintType() bool {
	return t.IsIntType() || t.IsUintType()
}

func (t StructFieldType) IsBoolType() bool {
	return t.TypeName == "bool"
}

func (t StructFieldType) IsByteArrayType() bool {
	return t.TypeName == "[]byte"
}

func (this *StructFieldType) GetSqlDescDefaultV() string {
	if this.IsIntOrUintType() {
		return "0"
	}
	return ""
}

func (this *StructFieldType) GetSqlCreateTableDeclare() string {
	v := quoteForSql(this.FiledName) + ` `
	if this.IsIntOrUintType() {
		v += `BIGINT NOT NULL DEFAULT 0`
	} else {
		v += `LONGBLOB NOT NULL DEFAULT ''`
	}
	return v
}

func listStructType(pkg *ast.Package, nameList []string) (list []StructType) {
	for _, file := range pkg.Files {
		for _, obj := range file.Scope.Objects {
			if obj.Kind != ast.Typ {
				continue
			}
			st, ok := obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType)
			if st == nil || ok == false {
				continue
			}
			var info StructType
			info.Name = obj.Name
			includeThisModel := false
			for _, name := range nameList {
				if info.Name == name {
					includeThisModel = true
					break
				}
			}
			if includeThisModel == false {
				continue
			}
			for _, one := range st.Fields.List {
				if len(one.Names) != 1 {
					continue
				}
				var unquoteTag string
				if one.Tag != nil {
					unquoteTag, _ = strconv.Unquote(one.Tag.Value)
				}
				thisFiled := StructFieldType{
					FiledName: one.Names[0].Name,
				}
				skipThisField := false
				reObj := regexp.MustCompile(`^this\.(\w+) *== *other\.(\w+)$`)
				for _, tag := range strings.Split(reflect.StructTag(unquoteTag).Get("korm"), `;`) {
					if skipThisField {
						break
					}
					tagPanicMessage := `MustParsePkg model.Name, field.FiledName: ` + info.Name + `,` + info.Name + ": " + strconv.Quote(tag)
					tagPrefix := strings.Split(tag, ":")
					if len(tagPrefix) > 2 {
						panic(tagPanicMessage)
					}
					switch tagPrefix[0] {
					case "":
					case "-":
						skipThisField = true
					case "primary":
						thisFiled.IsPrimaryKeyPart = true
					case "index":
						thisFiled.IndexList = append(thisFiled.IndexList, strings.Split(tagPrefix[1], ","))
					case "join":
						groupList := reObj.FindStringSubmatch(tagPrefix[1])
						if len(groupList) == 0 || thisFiled.LjField_This != "" {
							panic(tagPanicMessage)
						}
						thisFiled.LjField_This = groupList[1]
						thisFiled.LjFiled_Other = groupList[2]
					case "view":
						info.IsView = true
						if len(tagPrefix) != 2 {
							panic(tagPanicMessage)
						}
						info.ViewBeginD = tagPrefix[1]
					case "path":
						if len(tagPrefix) != 2 {
							panic(tagPanicMessage)
						}
						thisFiled.ViewJoinPath = strings.Split(strings.TrimSpace(tagPrefix[1]), ".")
					default:
						panic(`MustParsePkg.default model.Name, field.FiledName: ` + one.Names[0].Name + `,` + info.Name + ": " + strconv.Quote(tag))
					}
				}
				if skipThisField || info.Name == "_" {
					continue
				}
				tn, ok := one.Type.(*ast.Ident)
				if ok {
					thisFiled.TypeName = tn.Name
					info.FieldList = append(info.FieldList, thisFiled)
					continue
				}
				se, ok := one.Type.(*ast.SelectorExpr)
				if ok {
					pkgShort, ok := se.X.(*ast.Ident)
					if !ok {
						continue
					}
					thisFiled.TypeName = pkgShort.Name + "." + se.Sel.Name
					info.FieldList = append(info.FieldList, thisFiled)
					continue
				}
				se2, ok := one.Type.(*ast.StarExpr)
				if ok {
					thisFiled.TypeName = se2.X.(*ast.Ident).Name
					thisFiled.IsStarExpr = true
					info.FieldList = append(info.FieldList, thisFiled)
					continue
				}
				se3, ok := one.Type.(*ast.ArrayType)
				if ok && se3.Elt.(*ast.Ident).Name == `byte` {
					thisFiled.TypeName = "[]byte"
					info.FieldList = append(info.FieldList, thisFiled)
					continue
				}
			}
			if len(info.FieldList) == 0 {
				continue
			}
			for i, f := range info.FieldList {
				if i > 0 && f.IsPrimaryKeyPart && info.FieldList[i-1].IsPrimaryKeyPart == false {
					panic(`primary key part must in first: ` + strconv.Quote(info.Name))
				}
			}
			info.FieldList[0].IsPrimaryKeyPart = true
			checkIndexListInStruct(info)
			list = append(list, info)
		}
	}
	for idx := range list {
		info := &list[idx]
		if info.IsView == false {
			continue
		}
		var infoD *StructType
		for _, infoDp := range list {
			if infoDp.Name == info.ViewBeginD {
				infoD = &infoDp
				break
			}
		}
		if infoD == nil {
			panic("xxxxx " + info.ViewBeginD)
		}
		for idx0, f1 := range infoD.FieldList {
			if f1.IsPrimaryKeyPart == false {
				break
			}
			f0 := &info.FieldList[idx0]
			if f0.FiledName != f1.FiledName && (len(f0.ViewJoinPath) != 1 || f0.ViewJoinPath[0] != f1.FiledName) {
				panic("miss primary key in view " + strconv.Quote(info.Name))
			}
			f0.IsPrimaryKeyPart = true
		}
	}
	return list
}

type GoSourceWriter struct {
	importMap       map[string]bool // isHideImport
	buf             bytes.Buffer
	structMap       map[string]StructType
	req             MustCreateCode_Req
	leftJoinTypeMap map[string]struct{}
	orderByNameMap  map[string]bool
}

const ormAllName = `OrmAll`

func (this *GoSourceWriter) AddImportPath(importPath string) {
	this.importMap[importPath] = false
}

func (this *GoSourceWriter) AddImportPathHide(importPath string) {
	if _, ok := this.importMap[importPath]; ok {
		return
	}
	this.importMap[importPath] = true
}
func (this *GoSourceWriter) GetImportList() []string {
	var list []string
	for one := range this.importMap {
		list = append(list, one)
	}
	sort.Strings(list)
	return list
}

func (this *GoSourceWriter) FormatWriteSrc(to string) {
	buf2 := bytes.NewBuffer(nil)
	buf2.WriteString(`package ` + path.Base(path.Clean(this.req.ModelPkgFullPath)) + "\n")
	if len(this.importMap) > 0 {
		buf2.WriteString("import (\n")
		for _, one := range this.GetImportList() {
			isHideImport := this.importMap[one]
			if isHideImport {
				buf2.WriteString("_")
			}
			buf2.WriteString(strconv.Quote(one) + "\n")
		}
		buf2.WriteString(")\n")
	}
	buf2.WriteString("\n")
	buf2.Write(this.buf.Bytes())
	err := ioutil.WriteFile(to, buf2.Bytes(), 0777)
	if err != nil {
		panic(err)
	}
	src, err := format.Source(buf2.Bytes())
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(to, src, 0777)
	if err != nil {
		panic(err)
	}
}

func (this *GoSourceWriter) AddDeclear_ORM_Type(info StructType) {
	this.AddImportPath("bytes")
	this.buf.WriteString(`
type ` + info.GetOrmTypeName() + ` struct {
	supper *` + ormAllName + `
}

func (this *` + ormAllName + `) ` + info.Name + `() *` + info.GetOrmTypeName() + ` {
	return &` + info.GetOrmTypeName() + `{supper: this}
}
`)
}

const gAddWhereToBuf2 = `buf2.WriteString(this.bufWhere.String())`

type createCodeQuery_Req struct {
	arg0   string
	isExec bool
	ret0   string
}

func createCodeQuery(req createCodeQuery_Req) string {
	runExec := ``
	if req.isExec == false {
		runExec = `Query`
	}
	return req.ret0 + ` = this.supper.ExecRaw` + runExec + `(` + req.arg0 + ")\n"
}

func (this *GoSourceWriter) MustParsePkg(req MustCreateCode_Req) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), req.ModelPkgDir, nil, parser.AllErrors)
	if err != nil {
		panic(err)
	}
	this.structMap = map[string]StructType{}
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		for _, one := range listStructType(pkg, req.ModelNameList) {
			if _, ok := this.structMap[one.Name]; ok {
				panic(`MustCreateCode dump ` + one.Name)
			}
			this.structMap[one.Name] = one
		}
	}

	mustExistFieldIntOrString := func(st StructType, fieldName string) {
		for _, field := range st.FieldList {
			if field.IsStringOrIntUintType() == false {
				continue
			}
			if field.FiledName == fieldName {
				return
			}
		}
		panic(`mustExistFieldIntOrString not found int/string field ` + st.Name + `, ` + fieldName)
	}
	for _, name := range req.ModelNameList {
		model, ok := this.structMap[name]
		if ok == false {
			panic("MustCreateCode NotFound " + name)
		}

		for idx, field := range model.FieldList {
			if field.IsStarExpr == false {
				continue
			}
			otherModel, ok := this.structMap[field.TypeName]
			if ok == false {
				panic(`MustCreateCode Left join other table failed ` + strconv.Quote(model.Name+","+field.FiledName))
			}
			mustExistFieldIntOrString(model, field.LjField_This)
			mustExistFieldIntOrString(otherModel, field.LjFiled_Other)
			model.FieldList[idx] = field
		}
	}
}

func (this *GoSourceWriter) writeFnDecl(fnName string, retValue string) string {
	if this.req.GenMustFn {
		return `Must` + fnName + `() (` + retValue + `) {`
	}
	return fnName + `() (` + retValue + `, err error) {`
}

func (this *GoSourceWriter) writeMust() string {
	if this.req.GenMustFn {
		return "Must"
	}
	return ""
}

func (this *GoSourceWriter) writeFnDecl_SetInsert(fnName string, inArg string) string {
	if this.req.GenMustFn {
		return `Must` + fnName + `(` + inArg + `) {`
	}
	return fnName + `(` + inArg + `) (err error) {`
}

func (this *GoSourceWriter) writeErrDeclear_InReturn() string {
	if this.req.GenMustFn == false {
		return "err error"
	}
	return ""
}

func (this *GoSourceWriter) writeCheckErr(s string) string {
	data := `if err != nil {
`
	if this.req.GenMustFn {
		data += `panic(err)`
	} else {
		data += `return ` + s + ` err`
	}
	return data + "\n}"
}

func (this *GoSourceWriter) writeReturn(s string) string {
	if this.req.GenMustFn {
		return `
	return ` + s + `
`
	}
	return `
	return ` + s + `nil	
`
}

func (this *GoSourceWriter) writeErrDecl_IfMust() string {
	if this.req.GenMustFn == false {
		return ""
	}
	return "var err error\n"
}

func (this *GoSourceWriter) buildFn_Count_Exist(tName string, infoDotName string) {
	this.buf.WriteString(`
func (this * ` + tName + `) ` + this.writeFnDecl("Run_Count", "cnt int64"))
	this.buf.WriteString(`result, err := this.supper.ExecRawQuery(korm.BuildQueryStringCountExist(korm.BuildQueryStringCountExist_Req{
MainTableName: ` + strconv.Quote(infoDotName) + `,
MainTableNameAlias: this.joinNode.TableName,
RootInfoBufLeftJoin: &this.joinNode.Root.BufLeftJoin,
BufWhere: &this.bufWhere,
IsExist: false,
}), this.argsWhere...)
` + this.writeCheckErr("0, ") + `
	cnt, err = korm.ScanCount(result)
` + this.writeCheckErr("0, ") + `
	return cnt` + this.writeStr_IfMustValue_Equal(false, ", nil") + `
}

func (this *` + tName + `) ` + this.writeFnDecl("Run_Exist", "exist bool") + `
	result, err := this.supper.ExecRawQuery(korm.BuildQueryStringCountExist(korm.BuildQueryStringCountExist_Req{
		MainTableName: ` + strconv.Quote(infoDotName) + `,
		MainTableNameAlias: this.joinNode.TableName,
		RootInfoBufLeftJoin: &this.joinNode.Root.BufLeftJoin,
		BufWhere: &this.bufWhere,
		IsExist: true,
	}), this.argsWhere...)
` + this.writeCheckErr("false, ") + `
	exist, err = korm.ScanExist(result)
` + this.writeCheckErr("false, ") + `
return exist` + this.writeStr_IfMustValue_Equal(false, ",nil") + `
}
`)
}

func (this *GoSourceWriter) buildFn_ResultOne(tName string, infoDotName string) {
	this.buf.WriteString(`func (this *` + tName + `)` + this.writeFnDecl("Run_ResultOne", `info `+infoDotName))
	if this.req.GenMustFn {
		this.buf.WriteString(`info, _ = this.MustRun_ResultOne2()
	return info
`)
	} else {
		this.buf.WriteString(`info, _, err = this.Run_ResultOne2()
	return info, err
`)
	}
	this.buf.WriteString("}\n")
}

func (this *GoSourceWriter) buildFn_ResultOne2(tName string, info StructType) {
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
`)
}

func (this *GoSourceWriter) buildFn_ResultList(tName string, info StructType) {
	this.buf.WriteString(`
func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultList", "list []"+info.Name) + `
	` + info.BuildQueryString(this, `list,`) + `
	for result.Next() {
		var info ` + info.Name + `
	` + info.isD_CallFillNameList() + info.getScanAndParseFnCode(this, "list,") + `
		list = append(list, info)
	}
	return list` + this.writeStr_IfMustValue_Equal(false, `,nil`) + `
}`)
}

func (this *GoSourceWriter) buildFn_ResultMap(tName string, info StructType) {
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
` + info.isD_CallFillNameList() + info.getScanAndParseFnCode(this, "nil,") + `
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
}

func (this *GoSourceWriter) buildFn_ResultTotalMatch(tName string, info StructType) {
	this.buf.WriteString(`func (this *` + tName + `) ` + this.writeFnDecl("Run_ResultListWithTotalMatch", `list []`+info.Name+`, totalMatch int64`) + `
	` + this.writeStr_IfMustValue_Equal(true, `var err error`) + `	
	` + info.BuildQueryStringOnConn() + `
	defer result.Close()
	for result.Next() {
		var info ` + info.Name + `
		` + info.isD_CallFillNameList() + info.getScanAndParseFnCode(this, "list, totalMatch,") + `
		list = append(list, info)
	}
	result.Close()
		nextQuery := ""
		if this.supper.mode == korm.InitModeMysql {
			nextQuery = "select FOUND_ROWS()"
		} else if this.supper.mode == korm.InitModeSqlite {
			buf2.Reset()
			buf2.WriteString("SELECT COUNT(1) ")
			buf2.WriteString("FROM ` + quoteForSql(info.GetDotName()) + ` " + this.joinNode.TableName + " ")
			buf2.WriteString(this.` + func() string {
		if info.IsView {
			return "query."
		}
		return ""
	}() + `rootInfo.BufLeftJoin.String())
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

	return list, totalMatch` + this.writeStr_IfMustValue_Equal(false, ",nil") + `
}

`)
}

const (
	supportTypeInt    = "int"
	supportTypeUint   = "uint"
	supportTypeInt8   = `int8`
	supportTypeUint8  = `uint8`
	supportTypeInt16  = `int16`
	supportTypeUint16 = `uint16`
	supportTypeInt32  = `int32`
	supportTypeUint32 = `uint32`
	supportTypeInt64  = `int64`
	supportTypeUint64 = `uint64`
)
