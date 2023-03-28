package korm

import (
	"strconv"
)

func (this *GoSourceWriter) AddFunc_InitTable() {
	this.buf.WriteString(`type ` + prefix + this.writeStr_IfMustValue_Equal(true, "Must") + `NewDbMysqlReq struct{
	Addr	    string
	UserName    string
	Password    string
	UseDatabase string
}

func ` + prefix + this.writeStr_IfMustValue_Equal(true, "Must") + `NewDbMysql(req  ` + prefix + this.writeStr_IfMustValue_Equal(true, "Must") +
		`NewDbMysqlReq) (db *sql.DB` + this.writeStr_IfMustValue_Equal(false, ",err error") + `) {
	` + this.writeErrDecl_IfMust() + `
	db, err = sql.Open("mysql", req.UserName + ":" + req.Password + "@tcp("+req.Addr+")/")
	` + this.writeCheckErr("nil, ") + `
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + req.UseDatabase)
	` + this.writeCheckErr("nil, ") + `
	_ = db.Close()
	db, err = sql.Open("mysql", req.UserName + ":" + req.Password + "@tcp("+req.Addr+")/" + req.UseDatabase + "?charset=utf8")
	` + this.writeCheckErr("nil, ") + `
	return db ` + this.writeStr_IfMustValue_Equal(false, ", nil") + `
}
`)
	this.buf.WriteString(`func ` + prefix + this.writeMust() + `InitTableAll(db *sql.DB, mode string) ` + func() string {
		if this.req.GenMustFn {
			return ""
		}
		return "(err error)"
	}() + `{
`)
	this.buf.WriteString(this.writeErrDecl_IfMust())
	for _, name := range this.req.ModelNameList {
		st := this.structMap[name]
		if st.IsView {
			continue
		}
		this.AddImportPath("github.com/orestonce/korm")

		this.buf.WriteString(`err = korm.InitTable(korm.InitTableReq{
		Mode:      mode,
		TableName: ` + strconv.Quote(name) + `,
		FieldList: []korm.FieldSqlDefine{
`)
		for _, f := range st.FieldList {
			if f.IsStarExpr {
				continue
			}
			if f.IsIntOrUintType() {
				this.buf.WriteString(`{
				Type: korm.SqlTypeBigInt,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
			},`)
				continue
			}
			if f.IsBoolType() {
				this.buf.WriteString(`{
				Type: korm.SqlTypeBool,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
			},`)
				continue
			}
			if f.TypeName == "string" {
				if f.IsPrimaryKeyPart {
					this.buf.WriteString(`{
				Type: korm.SqlTypeChar255,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
			},`)
				} else {
					this.buf.WriteString(`{
				Type: korm.SqlTypeLongBlob,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
			},`)
				}
				continue
			}
			if f.IsBuildinTimeType() {
				this.buf.WriteString(`{
				Type: korm.SqlTypeChar255,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
},`)
				continue
			}
			this.buf.WriteString(`{
				Type: korm.SqlTypeLongBlob,
				Name: ` + strconv.Quote(f.FiledName) + `,
				IsPrimaryKey: ` + strconv.FormatBool(f.IsPrimaryKeyPart) + `,
			},`)
		}
		this.buf.WriteString(`
},
` + indexListToDeclString(st) + `Db: db,
	})
` + this.writeCheckErr("") + `
`)
	}
	if this.req.GenMustFn == false {
		this.buf.WriteString(`return nil`)
	}
	this.buf.WriteString(`
}
`)
}

func (this *GoSourceWriter) AddFunc_DbType() {
	this.AddImportPath("database/sql")
	this.buf.WriteString("type " + ormAllName + " struct {\n")
	this.buf.WriteString(`db *sql.DB	// db, tx任选其一
	tx *sql.Tx
	mode string	// sqlite, mysql
	}
func (this *` + ormAllName + `) ExecRawQuery(query string, args ...interface{}) (*sql.Rows, error) {
	if this.db != nil {
		return this.db.Query(query, args...)
	} else if this.tx != nil {
		return this.tx.Query(query, args...)
	}
	panic("ExecRawQuery: OrmAll must include db or tx")
}

func OrmAllNew(db *sql.DB, mode string) *` + ormAllName + ` {
	return &` + ormAllName + `{
		db: db,
		mode: mode,
	}
}

func (this *` + ormAllName + `) ExecRaw(query string, args ...interface{}) (sql.Result, error) {
	if this.db != nil {
		return this.db.Exec(query, args...)
	} else if this.tx != nil {
		return this.tx.Exec(query, args...)
	}
	panic("ExecRaw: OrmAll must include db or tx")
}

func (this *` + ormAllName + `) MustTxCallback(cb func(db *OrmAll)) {
	if this.tx != nil {
		panic("MustSingleThreadTxCallback repeat call")
	}
	tx, err := this.db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	cb(&OrmAll{
		tx: tx,
		mode: this.mode,
	})
	err = tx.Commit()
	if err != nil {
		panic(err)
	}
}
`)

}
