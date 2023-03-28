package korm

import (
	"bytes"
	"strconv"
)

type KORM_leftJoinRootInfo struct {
	BufLeftJoin  bytes.Buffer
	TableNameIdx int
}

type KORM_leftJoinNode struct {
	ThisLevelJoinList   []*KORM_leftJoinNode
	FieldName           string
	TableName           string // _0, _1, _2
	Root                *KORM_leftJoinRootInfo
	SelectFieldNameList []string
}

func (this *KORM_leftJoinNode) writeToBuf(buf2 *bytes.Buffer) {
	for idx, FieldName := range this.SelectFieldNameList {
		if idx > 0 {
			buf2.WriteString(", ")
		}
		buf2.WriteString(this.TableName + ".`" + FieldName + "`")
	}
	buf2.WriteString(" ")
}
func (this *KORM_leftJoinNode) FillSelect(buf2 *bytes.Buffer, isFirst bool) {
	for idx, FieldName := range this.SelectFieldNameList {
		if idx > 0 || isFirst == false {
			buf2.WriteString(", ")
		}
		buf2.WriteString(this.TableName + ".`" + FieldName + "`")
		isFirst = false
	}
	for _, sub := range this.ThisLevelJoinList {
		sub.FillSelect(buf2, isFirst)
	}
}
func (this *KORM_leftJoinNode) AddLeftJoin(rightTableName string, thisFieldName string, leftFieldName string, rightFieldName string) *KORM_leftJoinNode {
	for _, one := range this.ThisLevelJoinList {
		if one.FieldName == thisFieldName {
			return one
		}
	}
	newNode := &KORM_leftJoinNode{
		FieldName: thisFieldName,
		TableName: "_" + strconv.Itoa(this.Root.TableNameIdx),
		Root:      this.Root,
	}
	this.Root.TableNameIdx++
	this.ThisLevelJoinList = append(this.ThisLevelJoinList, newNode)
	this.Root.BufLeftJoin.WriteString("LEFT JOIN `" + rightTableName + "` " + newNode.TableName + " ON " + this.TableName + ".`" + leftFieldName + "` = " + newNode.TableName + ".`" + rightFieldName + "` ")
	return newNode
}
