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
	joinNode    *` + strings.ToLower(prefix) + `leftJoinNode
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
node := this.joinNode.addLeftJoin("` + f.TypeName + `", ` + strconv.Quote(f.FiledName) + `, "` + f.LjField_This + `", "` + f.LjFiled_Other + `")
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

func (this *GoSourceWriter) addLeftJoin() {
	nodeName := this.getLeftJoinNodeName()
	rootName := this.getLeftJoinRootInfoName()

	this.buf.WriteString(`
type ` + rootName + ` struct {
	bufLeftJoin  bytes.Buffer
	tableNameIdx int
}

type ` + nodeName + ` struct {
	thisLevelJoinList []*` + nodeName + `
	fieldName         string
	tableName         string // _0, _1, _2
	root              *` + rootName + `
    selectFieldNameList []string
}
func (this *` + nodeName + `) writeToBuf(buf2 *bytes.Buffer) {
	for idx, fieldName := range this.selectFieldNameList {
		if idx > 0 {
			buf2.WriteString(", ")
		}
		buf2.WriteString(this.tableName + ".` + "`" + `" + fieldName + "` + "`" + `")
	}
	buf2.WriteString(" ")
}
func (this *` + this.getLeftJoinNodeName() + `) fillSelect(buf2 *bytes.Buffer, isFirst bool) {
	for idx, fieldName := range this.selectFieldNameList {
		if idx > 0 || isFirst == false {
			buf2.WriteString(", ")
		}
		buf2.WriteString(this.tableName + ".` + "`" + `" + fieldName + "` + "`" + `")
	}
	for _, sub := range this.thisLevelJoinList {
		sub.fillSelect(buf2, false)
	}
}
func (this *korm_leftJoinNode) addLeftJoin(rightTableName string, thisFieldName string, leftFieldName string, rightFieldName string) *korm_leftJoinNode {
	for _, one := range this.thisLevelJoinList {
		if one.fieldName == thisFieldName {
			return one
		}
	}
	newNode := &` + this.getLeftJoinNodeName() + `{
		fieldName: thisFieldName,
		tableName: "_" + strconv.Itoa(this.root.tableNameIdx),
		root:      this.root,
	}
	this.root.tableNameIdx++
	this.thisLevelJoinList = append(this.thisLevelJoinList, newNode)
	this.root.bufLeftJoin.WriteString("LEFT JOIN ` + "`" + `" + rightTableName + "` + "`" + ` " + newNode.tableName + " ON " + this.tableName + ".` + "`" + `" + leftFieldName + "` + "`" + ` = " + newNode.tableName + ".` + "`" + `" + rightFieldName + "` + "`" + ` ")
	return newNode
}
`)
}

func (this *StructType) getFNName_fillSelectFieldNameList() string {
	return strings.ToLower(prefix) + `fillSelectFieldNameList_` + this.Name
}

func (this *GoSourceWriter) AddFunc_fillSelectFieldNameList(info StructType) {
	this.buf.WriteString(`func ` + info.getFNName_fillSelectFieldNameList() + `(joinNode *` + this.getLeftJoinNodeName() + `) {
		joinNode.selectFieldNameList = []string{`)
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
	for _, sub := range joinNode.thisLevelJoinList {
		switch sub.fieldName {
		default:
			panic(` + strconv.Quote(info.getFNName_fillSelectFieldNameList()) + ` + strconv.Quote(sub.fieldName))
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
