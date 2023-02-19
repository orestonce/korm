package korm

import "strconv"

func (this *GoSourceWriter) ChangeLinkOpBegin(name string, op string, req AddWhereAllField_Req) {
	this.buf.WriteString(`func (this *` + name + `) CondMultOpBegin_` + op + `() *` + name + `{
	if this.bufWhere.Len() > 0 {
		if ` + req.isLinkBegin + ` == false {
			this.bufWhere.WriteString(` + req.linkOpList + `[0] + " ")
		}
	} else {
		this.bufWhere.WriteString("WHERE ")
	}
	this.bufWhere.WriteString("( ")
	` + req.linkOpList + ` = append([]string{` + strconv.Quote(op) + `}, ` + req.linkOpList + `...)
	` + req.isLinkBegin + ` = true
	return this
}
`)
}

func (this *GoSourceWriter) ChangeLinkOpEnd(name string, req AddWhereAllField_Req) {
	this.buf.WriteString(`func (this *` + name + `) CondMultOpEnd() *` + name + `{
	if ` + req.isLinkBegin + ` {
		panic("() is not allowed in sql statement")	// bad sql: SELECT * FROM u where ()
	}
	` + req.linkOpList + ` = ` + req.linkOpList + `[1:]
	this.bufWhere.WriteString(") ")
	return this
}
`)
}
