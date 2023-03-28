package korm

func (this *GoSourceWriter) AddFunc_ORM_Update(info StructType) {
	this.buf.WriteString(`// Update ` + info.Name + "\n")
	tName := info.GetOrmSubType_UpdateName()
	this.buf.WriteString(`type ` + tName + ` struct {
	supper *` + ormAllName + `
	bufWhere bytes.Buffer
	argsWhere []interface{}
	bufSet   bytes.Buffer
	argsSet []interface{}
	linkOpList []string
	isLinkBegin bool
}

func (this *` + info.GetOrmTypeName() + `) Update() *` + tName + `{
	return &` + tName + `{supper: this.supper, linkOpList: []string{"AND"}, isLinkBegin: true}
}
`)
	fnName := `UpdateBy`
	for _, f := range info.FieldList {
		if f.IsPrimaryKeyPart {
			fnName += "_" + f.FiledName
		}
	}
	this.buf.WriteString(`func (this *` + info.GetOrmTypeName() + ") " + this.writeMust() + fnName + "(info " + info.Name + ") (rowsAffected int64, " +
		this.writeErrDeclear_InReturn() + ") {\n")
	if this.req.GenMustFn {
		this.buf.WriteString("rowsAffected = ")
	} else {
		this.buf.WriteString("rowsAffected, err = ")
	}
	this.buf.WriteString(`this.Update()`)
	for _, f := range info.FieldList {
		if f.IsPrimaryKeyPart == false {
			break
		}
		this.buf.WriteString(`.Where_` + f.FiledName + `().Equal(info.` + f.FiledName + `)`)
	}
	for _, f := range info.FieldList {
		if f.IsStarExpr || f.IsPrimaryKeyPart {
			continue
		}
		this.buf.WriteString(`.Set_` + f.FiledName + `(info.` + f.FiledName + `)`)
	}
	if this.req.GenMustFn {
		this.buf.WriteString(".MustRun()\nreturn rowsAffected")
	} else {
		this.buf.WriteString(".Run()\nreturn rowsAffected, err")
	}
	this.buf.WriteString(`
}
`)
	for _, f := range info.FieldList {
		if f.IsStarExpr || f.IsPrimaryKeyPart {
			continue
		}
		if f.IsIntType() {
			this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) Inc_` + f.FiledName + `(v int) *` + info.GetOrmSubType_UpdateName() + `{
	if len(this.argsSet) > 0 {
		this.bufSet.WriteString(", ")
	} else {
		this.bufSet.WriteString("SET ")
	}
	this.bufSet.WriteString("` + quoteForSql(f.FiledName) + ` = ` + quoteForSql(f.FiledName) + ` + ? ")
	this.argsSet = append(this.argsSet, v)
	return this 
}
`)
		}
		if f.IsStringOrIntUintType() || f.IsByteArrayType() {
			this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) Set_` + f.FiledName + "(" + f.FiledName + " " + f.TypeName + ") *" + info.GetOrmSubType_UpdateName() + `{
	if len(this.argsSet) > 0 {
		this.bufSet.WriteString(", ")
	} else {
		this.bufSet.WriteString("SET ")
	}
	this.bufSet.WriteString("` + quoteForSql(f.FiledName) + ` = ? ")
	this.argsSet = append(this.argsSet, `)
			if f.IsByteArrayType() {
				this.buf.WriteString("string(" + f.FiledName + ")")
			} else {
				this.buf.WriteString(f.FiledName)
			}
			this.buf.WriteString(`)
	return this
}
`)
		} else if f.IsBoolType() {
			this.AddImportPath("strconv")
			this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) Set_` + f.FiledName + "(" + f.FiledName + " " + f.TypeName + ") *" + info.GetOrmSubType_UpdateName() + `{
	if len(this.argsSet) > 0 {
		this.bufSet.WriteString(", ")
	} else {
		this.bufSet.WriteString("SET ")
	}
	this.bufSet.WriteString("` + quoteForSql(f.FiledName) + ` = ? ")
	this.argsSet = append(this.argsSet, strconv.FormatBool(` + f.FiledName + `))
	return this
}
`)
		} else if f.IsBuildinTimeType() {
			this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) Set_` + f.FiledName + "(" + f.FiledName + " " + f.TypeName + ") *" + info.GetOrmSubType_UpdateName() + `{
	if len(this.argsSet) > 0 {
		this.bufSet.WriteString(", ")
	} else {
		this.bufSet.WriteString("SET ")
	}
	this.bufSet.WriteString("` + quoteForSql(f.FiledName) + ` = ? ")
	this.argsSet = append(this.argsSet, ` + f.FiledName + `.UTC().Format(time.RFC3339Nano))
	return this
}
`)
		} else {
			this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) Set_` + f.FiledName + "(" + f.FiledName + " " + f.TypeName + ") *" + info.GetOrmSubType_UpdateName() + `{
	if len(this.argsSet) > 0 {
		this.bufSet.WriteString(", ")
	} else {
		this.bufSet.WriteString("SET ")
	}
	v` + f.FiledName + `, err := json.Marshal(` + f.FiledName + `)
	if err != nil {
		panic(err)
	}
	this.bufSet.WriteString("` + quoteForSql(f.FiledName) + ` = ? ")
	this.argsSet = append(this.argsSet, v` + f.FiledName + `)
	return this
}
`)
		}
	}
	this.AddImportPath("strconv")
	this.buf.WriteString(`func (this *` + info.GetOrmSubType_UpdateName() + `) ` + this.writeFnDecl("Run", "RowsAffected int64") + `
	if len(this.argsSet) == 0 {
		panic("len(this.argsSet) == 0")
	}
	var buf2 bytes.Buffer
	buf2.WriteString("UPDATE ` + quoteForSql(info.Name) + ` ")
	buf2.WriteString(this.bufSet.String())
` + gAddWhereToBuf2 + `
	var result sql.Result
	` + this.writeErrDecl_IfMust() + `
` + createCodeQuery(createCodeQuery_Req{
		arg0:   "buf2.String(), append(this.argsSet, this.argsWhere...)...",
		isExec: true,
		ret0:   "result, err",
	}) + this.writeCheckErr("0,") + `
	RowsAffected, err = result.RowsAffected()
	` + this.writeCheckErr("0,") + `
	return RowsAffected` + this.writeStr_IfMustValue_Equal(false, ",nil") + `
}
`)
}

func (t StructType) GetOrmSubType_UpdateName() string {
	return prefix + t.Name + "_UpdateObj"
}
