package korm

import (
	"strconv"
	"strings"
)

func (info *StructType) getScanAndParseFnCode(this *GoSourceWriter, s string) string {
	return `resp := ` + info.getScanFnName() + `(` + func() string {
		if info.IsView {
			return ""
		}
		return `this.joinNode, `
	}() + `&info)
		err = result.Scan(resp.argList...)
		` + this.writeCheckErr(s) + `
		for idx, a := range resp.argList {
			v := a.(*sql.NullString).String
			if v == "" {
				continue
			}
			err = resp.argParseFn[idx](v)
			` + this.writeCheckErr(s) + `	
		}
`
}

func (this *GoSourceWriter) writeScanInfoCode() {
	respName := strings.ToLower(prefix) + `scan_resp`
	this.buf.WriteString(`type ` + respName + ` struct {
	argList []interface{}
	argParseFn []func(v string)(err error)
}
`)
	writeScanCodeForF := func(f StructFieldType) {
		this.buf.WriteString(`{
resp.argList = append(resp.argList, new(sql.NullString))
tmpFn := func(v string) (err error) {
`)
		buf := &this.buf
		if f.TypeName == `string` {
			buf.WriteString(`info.` + f.FiledName + ` = v
`)
		} else if f.IsByteArrayType() {
			buf.WriteString("info." + f.FiledName + ` = []byte(v)
`)
		} else if f.IsIntType() {
			buf.WriteString(`vi, err := strconv.ParseInt(v, 10, 64)
if err != nil {
	return err
}
		` + `info.` + f.FiledName + ` = ` + f.TypeName + `(vi)
`)
		} else if f.IsUintType() {
			buf.WriteString(`vu, err := strconv.ParseUint(v, 10, 64)
if err != nil {
	return err
}
		` + `info.` + f.FiledName + ` = ` + f.TypeName + `(vu)
`)
		} else if f.IsBuildinTimeType() {
			buf.WriteString(`vt, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			return err
		}
		` + `info.` + f.FiledName + ` = vt
`)
		} else {
			this.AddImportPath("encoding/json")
			buf.WriteString(`err = json.Unmarshal([]byte(v)` + `,&` + `info.` + f.FiledName + `)
			if err != nil {
				return err
			}
`)
		}
		buf.WriteString(`
return nil
}
resp.argParseFn = append(resp.argParseFn, tmpFn)
}
`)
	}
	for _, name := range this.req.ModelNameList {
		info := this.structMap[name]

		if info.IsView {
			this.buf.WriteString(`func ` + info.getScanFnName() + `(info *` + info.Name + `) (resp ` + respName + `) {
`)
			for _, f := range info.FieldList {
				this.buf.WriteString("// " + f.FiledName + "\n")
				writeScanCodeForF(f)
			}
			this.buf.WriteString(`
return resp
}
`)
			continue
		}
		this.buf.WriteString(`func ` + info.getScanFnName() + `(joinNode *` + this.getLeftJoinNodeName() + `, info *` + info.Name + `) (resp ` + respName + `) {
`)
		var buf = &this.buf
		buf.WriteString(`for _, one := range joinNode.SelectFieldNameList {
		switch one {
		default:
			panic("GetScanInfoCode error" + strconv.Quote(one))
		`)
		for _, f := range info.FieldList {
			if f.IsStarExpr {
				continue
			}
			buf.WriteString(`case ` + strconv.Quote(f.FiledName) + `:
`)
			writeScanCodeForF(f)
		}
		buf.WriteString(`}
	}
`)
		this.buf.WriteString(`for _, sub := range joinNode.ThisLevelJoinList {
		switch sub.FieldName {
		default:
			panic(` + strconv.Quote(info.Name) + `)
`)
		for _, f := range info.FieldList {
			if f.IsStarExpr == false {
				continue
			}
			otherS := this.structMap[f.TypeName]
			this.buf.WriteString(`case ` + strconv.Quote(f.FiledName) + `:
			info.` + f.FiledName + ` = &` + f.TypeName + `{}
			resp0 := ` + otherS.getScanFnName() + `(sub, info.` + f.FiledName + `)
			resp.argList = append(resp.argList, resp0.argList...)
			resp.argParseFn = append(resp.argParseFn, resp0.argParseFn...)
`)
		}
		this.buf.WriteString(`}
	}
return resp
}
`)
	}

	this.buf.WriteString("")
}

func (info StructType) getScanFnName() string {
	return strings.ToLower(prefix) + `` + info.Name + `_scan`
}

func (this *GoSourceWriter) getLeftJoinNodeName() string {
	return "korm.KORM_leftJoinNode"
}

func (this *GoSourceWriter) getLeftJoinRootInfoName() string {
	return "korm.KORM_leftJoinRootInfo"
}
