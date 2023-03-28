package korm

import (
	"strconv"
	"strings"
)

func (this *GoSourceWriter) defineLeftJoinType(ljName string, info StructType) {
	_, ok := this.leftJoinTypeMap[ljName]
	if ok {
		return
	}
	this.leftJoinTypeMap[ljName] = struct{}{}
	this.buf.WriteString(`type ` + ljName + ` struct {
	joinNode    *` + this.getLeftJoinNodeName() + `
	bufWhere   *bytes.Buffer
	argsWhereP  *[]interface{}
	isLinkBeginP *bool
	linkOpListP *[]string
	orderByP    *[]string
}
`)
	this.AddWhereAllField(info, ljName, true, true)
	this.AddFuncOrderBy(ljName, info, true)

	for _, f := range info.FieldList {
		if f.IsStarExpr == false {
			continue
		}
		otherS := this.structMap[f.TypeName]
		otherSLjName := otherS.GetLeftJoinName()

		this.buf.WriteString(`func (this *` + ljName + `) LeftJoin_` + f.FiledName + `() *` + otherSLjName + `{
node := this.joinNode.AddLeftJoin("` + f.TypeName + `", ` + strconv.Quote(f.FiledName) + `, "` + f.LjField_This + `", "` + f.LjFiled_Other + `")
		return &` + otherSLjName + ` {
			joinNode:	  node,
			bufWhere:     this.bufWhere,
			argsWhereP:   this.argsWhereP,
			isLinkBeginP: this.isLinkBeginP,
			linkOpListP:  this.linkOpListP,
			orderByP:     this.orderByP,
		}
}
`)
		this.defineLeftJoinType(otherSLjName, otherS)
	}
}

func (this *StructType) getFNName_fillSelectFieldNameList() string {
	return strings.ToLower(prefix) + `fillSelectFieldNameList_` + this.Name
}

func (this *GoSourceWriter) AddFunc_fillSelectFieldNameList(info StructType) {
	this.buf.WriteString(`func ` + info.getFNName_fillSelectFieldNameList() + `(joinNode *` + this.getLeftJoinNodeName() + `) {
		joinNode.SelectFieldNameList = []string{`)
	for idx, f := range info.FieldList {
		if f.IsStarExpr {
			continue
		}
		if idx > 0 {
			this.buf.WriteString(`, `)
		}
		this.buf.WriteString(strconv.Quote(f.FiledName))
	}
	this.buf.WriteString(`}
	for _, sub := range joinNode.ThisLevelJoinList {
		switch sub.FieldName {
		default:
			panic(` + strconv.Quote(info.getFNName_fillSelectFieldNameList()) + ` + strconv.Quote(sub.FieldName))
		`)
	for _, f := range info.FieldList {
		if f.IsStarExpr == false {
			continue
		}
		this.buf.WriteString(`case ` + strconv.Quote(f.FiledName) + `:
`)

		otherS := this.structMap[f.TypeName]
		this.buf.WriteString(otherS.getFNName_fillSelectFieldNameList() + `(sub)
`)
	}
	this.buf.WriteString(`}
	}
}
`)
}

func (t StructType) GetLeftJoinName() string {
	return prefix + t.Name + "_BeLeftJoin"
}
