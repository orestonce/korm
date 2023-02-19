package korm

import (
	"bytes"
	"strings"
)

func (this *GoSourceWriter) AddFunc_ORM_Create(info StructType) {
	for _, one := range [][2]string{
		{"Insert", "INSERT"},
		{"Set", "REPLACE"},
	} {
		fnNamePart := one[0]
		sqlPart := one[1]

		this.buf.WriteString(`func (this *` + info.GetOrmTypeName() + ")" + this.writeFnDecl_SetInsert(fnNamePart, "info "+info.Name) + `
` + this.writeErrDecl_IfMust())
		var varNameList []string
		for _, f := range info.FieldList {
			if f.IsStarExpr {
				continue
			}
			if f.IsStringOrIntUintType() {
				varNameList = append(varNameList, "info."+f.FiledName)
			} else if f.IsByteArrayType() {
				varNameList = append(varNameList, "string(info."+f.FiledName+")") // 避免nil被sql解析成NULL
			} else if f.IsBuildinTimeType() {
				this.AddImportPath("time")
				this.buf.WriteString(`v` + f.FiledName + ` := info.` + f.FiledName + `.UTC().Format(time.RFC3339Nano)` + "\n")
				varNameList = append(varNameList, "v"+f.FiledName)
			} else if f.IsBoolType() {
				this.AddImportPath("strconv")
				this.buf.WriteString(`v` + f.FiledName + ` := strconv.FormatBool(info.` + f.FiledName + `)
`)
				varNameList = append(varNameList, "v"+f.FiledName)
			} else {
				this.AddImportPath("encoding/json")
				this.buf.WriteString(`v` + f.FiledName + `, err := json.Marshal(info.` + f.FiledName + `)
` + this.writeCheckErr("") + `
`)
				varNameList = append(varNameList, "v"+f.FiledName)
			}
		}
		var buf0 bytes.Buffer
		buf0.WriteString(`"` + sqlPart + ` INTO ` + quoteForSql(info.Name) + "(")
		for idx, f := range info.FieldList {
			if f.IsStarExpr {
				continue
			}
			if idx > 0 {
				buf0.WriteString(",")
			}
			buf0.WriteString(quoteForSql(f.FiledName) + " ")
		}
		buf0.WriteString(") VALUES(")
		for idx, f := range info.FieldList {
			if f.IsStarExpr {
				continue
			}
			if idx > 0 {
				buf0.WriteString(",")
			}
			buf0.WriteString("?")
		}
		buf0.WriteString(`)"`)
		this.buf.WriteString(createCodeQuery(createCodeQuery_Req{
			arg0:   buf0.String() + "," + strings.Join(varNameList, ","),
			isExec: true,
			ret0:   "_, err",
		}))
		this.buf.WriteString(this.writeCheckErr("") + "\n" + this.writeReturn("") + `}
`)
	}
}
