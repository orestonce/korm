package korm

import (
	"strconv"
	"strings"
)

func (info *StructType) getScanAndParseFnCode(this *GoSourceWriter, s string) string {
	return `resp := ` + info.getScanFnName() + `(this.joinNode, &info)
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
	for _, name := range this.req.ModelNameList {
		info := this.structMap[name]
		this.buf.WriteString(`func ` + info.getScanFnName() + `(joinNode *` + this.getLeftJoinNodeName() + `, info *` + info.Name + `) (resp ` + respName + `) {
`)
		var buf = &this.buf
		buf.WriteString(`for _, one := range joinNode.selectFieldNameList {
		switch one {
		default:
			panic("GetScanInfoCode error" + strconv.Quote(one))
		`)
		for _, f := range info.FieldList {
			if f.IsStarExpr {
				continue
			}
			buf.WriteString(`case ` + strconv.Quote(f.FiledName) + `:
		resp.argList = append(resp.argList, new(sql.NullString))
		tmpFn := func(v string) (err error) {
`)
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
`)
		}
		buf.WriteString(`}
	}
`)
		this.buf.WriteString(`for _, sub := range joinNode.thisLevelJoinList {
		switch sub.fieldName {
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
`)
		this.buf.WriteString(`return resp
}
`)
	}
}

func (info StructType) getScanFnName() string {
	return strings.ToLower(prefix) + `` + info.Name + `_scan`
}

func (this *GoSourceWriter) getLeftJoinNodeName() string {
	return strings.ToLower(prefix) + `leftJoinNode`
}

func (this *GoSourceWriter) getLeftJoinRootInfoName() string {
	return strings.ToLower(prefix) + `leftJoinRootInfo`
}
