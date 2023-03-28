package korm

import (
	"bytes"
	"database/sql"
	"errors"
)

type BuildQueryStringCountExist_Req struct {
	MainTableName       string
	MainTableNameAlias  string
	RootInfoBufLeftJoin *bytes.Buffer
	BufWhere            *bytes.Buffer
	IsExist             bool
}

func BuildQueryStringCountExist(req BuildQueryStringCountExist_Req) string {
	var buf2 bytes.Buffer
	buf2.WriteString("SELECT COUNT(1) FROM `" + req.MainTableName + "` " + req.MainTableNameAlias + " ")
	buf2.WriteString(req.RootInfoBufLeftJoin.String())
	buf2.WriteString(req.BufWhere.String())
	if req.IsExist {
		buf2.WriteString(" LIMIT 1")
	}
	return buf2.String()
}

func ScanCount(result *sql.Rows) (cnt int64, err error) {
	defer result.Close()
	if result.Next() == false {
		return 0, errors.New("ScanCount: not fount result")
	}
	err = result.Scan(&cnt)
	if err != nil {
		return 0, err
	}
	return cnt, nil
}

func ScanExist(result *sql.Rows) (exist bool, err error) {
	cnt, err := ScanCount(result)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}
