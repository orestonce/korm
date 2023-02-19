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
	source.addLeftJoin()
	for _, name := range req.ModelNameList {
		info := source.structMap[name]
		source.AddDeclear_ORM_Type(info)
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
	Name      string
	FieldList []StructFieldType
}

func (t StructType) GetOrmTypeName() string {
	return prefix + t.Name
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
					tagPrefix := strings.Split(tag, ":")
					if len(tagPrefix) > 2 {
						panic(`MustParsePkg.0 ` + strconv.Quote(tag))
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
							panic(`MustParsePkg model.Name, field.FiledName: ` + one.Names[0].Name + `,` + info.Name + ": " + strconv.Quote(tag))
						}
						thisFiled.LjField_This = groupList[1]
						thisFiled.LjFiled_Other = groupList[2]
					default:
						panic(`MustParsePkg.default model.Name, field.FiledName: ` + one.Names[0].Name + `,` + info.Name + ": " + strconv.Quote(tag))
					}
				}
				if skipThisField {
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
	return data + "\n}\n"
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
