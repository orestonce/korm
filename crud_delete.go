package korm

func (this *GoSourceWriter) AddFunc_ORM_Delete(info StructType) {
	tName := info.GetOrmSubType_Delete()
	this.buf.WriteString(`type ` + tName + ` struct{
	supper *` + ormAllName + `
	bufWhere bytes.Buffer
	argsWhere []interface{}
	bufSet   bytes.Buffer
	argsSet []interface{}
	orderBy []string
	linkOpList []string
	isLinkBegin bool
}

func (this *` + info.GetOrmTypeName() + `) Delete() *` + tName + ` {
	return &` + tName + `{supper: this.supper, linkOpList: []string{"AND"}, isLinkBegin: true}
}
`)
	this.buf.WriteString(`func (this *` + tName + `) ` + this.writeFnDecl("Run", "RowsAffected int64") + `
	var buf2 bytes.Buffer
	buf2.WriteString("DELETE FROM ` + info.Name + ` ")
	` + gAddWhereToBuf2 + `
	var result sql.Result
	` + this.writeErrDecl_IfMust())
	this.buf.WriteString(createCodeQuery(createCodeQuery_Req{
		arg0:   "buf2.String(), this.argsWhere...",
		isExec: true,
		ret0:   "result, err",
	}) + `
	` + this.writeCheckErr("0, ") + `
	RowsAffected, err = result.RowsAffected()
` + this.writeCheckErr("0,") + `
	return RowsAffected` + this.writeStr_IfMustValue_Equal(false, ", nil") + `
}
`)
}

func (t StructType) GetOrmSubType_Delete() string {
	return prefix + t.Name + "_DeleteObj"
}
