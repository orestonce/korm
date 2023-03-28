package korm

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	SqlTypeChar255  = "CHAR(255)"
	SqlTypeBigInt   = "bigint"
	SqlTypeLongBlob = "longblob"
	SqlTypeBool     = "bool"
)

const (
	InitModeMysql  = "mysql"
	InitModeSqlite = "sqlite"
)

type FieldSqlDefine struct {
	Type         string
	Name         string
	IsPrimaryKey bool
}

type InitTableReq struct {
	Mode      string
	TableName string
	FieldList []FieldSqlDefine
	IndexList [][]string
	Db        *sql.DB
}

func InitTable(req InitTableReq) (err error) {
	if len(req.FieldList) == 0 || req.FieldList[0].IsPrimaryKey == false {
		return errors.New("FieldList must has value, first field must IsPrimaryKey, TableName " + req.TableName)
	}
	var stringOrLongBlobFieldNameList []string
	for i, f := range req.FieldList {
		if i > 0 && f.IsPrimaryKey && req.FieldList[i-1].IsPrimaryKey == false {
			return errors.New("primary key must at first, " + req.TableName)
		}
		if f.Type == SqlTypeChar255 || f.Type == SqlTypeLongBlob {
			stringOrLongBlobFieldNameList = append(stringOrLongBlobFieldNameList, f.Name)
		}
	}
	getFieldDefine := func(f FieldSqlDefine) (s string, err error) {
		s = "`" + f.Name + "` "
		switch f.Type {
		case SqlTypeChar255:
			s = s + f.Type + " NOT NULL DEFAULT '' "
		case SqlTypeLongBlob:
			s = s + f.Type + " NOT NULL "
		case SqlTypeBigInt:
			s = s + f.Type + " NOT NULL DEFAULT 0 "
		case SqlTypeBool:
			if req.Mode == InitModeMysql {
				s = s + "ENUM('false', 'true') NOT NULL DEFAULT 'false' "
			} else if req.Mode == InitModeSqlite {
				s = s + "CHAR(255) CHECK( " + "`" + f.Name + "`" + " IN ('false','true')) NOT NULL DEFAULT 'false' "
			} else {
				return "", errors.New("invalid Mode " + req.Mode)
			}
		default:
			return "", errors.New("invalid f.Type " + f.Type)
		}
		return s, nil
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString("CREATE TABLE IF NOT EXISTS `" + req.TableName + "`(")
	for idx, f := range req.FieldList {
		if idx > 0 {
			buf.WriteString(",")
		}
		var s string
		s, err = getFieldDefine(f)
		if err != nil {
			return err
		}
		buf.WriteString(s)
	}
	buf.WriteString(", PRIMARY KEY (")
	for idx, f := range req.FieldList {
		if f.IsPrimaryKey == false {
			break
		}
		if idx > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(f.Name)
	}
	buf.WriteString(")")
	if req.Mode == InitModeMysql {
		indexInCreateSql(buf, req.TableName, req.IndexList, stringOrLongBlobFieldNameList)
	}
	buf.WriteString(")")
	createTableSql := buf.String()
	_, err = req.Db.Exec(createTableSql)
	if err != nil {
		return errors.New("Db.Exec err " + err.Error() + "\n" + createTableSql)
	}
	type FieldDefInDb struct {
		Name         string
		Type         string
		IsPrimaryKey bool
	}
	var fieldListInDb []FieldDefInDb
	var expectTypeMap map[string]string

	if req.Mode == InitModeMysql {
		expectTypeMap = map[string]string{
			SqlTypeChar255:  "char(255)",
			SqlTypeLongBlob: "longblob",
			SqlTypeBigInt:   "bigint",
			SqlTypeBool:     "enum('false','true')",
		}
		var rows *sql.Rows
		rows, err = req.Db.Query("DESC `" + req.TableName + "`")
		if err != nil {
			return err
		}
		for rows.Next() {
			var Field sql.NullString
			var Type sql.NullString
			var Null sql.NullString
			var Key sql.NullString
			var Default sql.NullString
			var Extra sql.NullString
			err = rows.Scan(&Field, &Type, &Null, &Key, &Default, &Extra)
			if err != nil {
				rows.Close()
				return err
			}
			fieldListInDb = append(fieldListInDb, FieldDefInDb{
				Name:         Field.String,
				Type:         Type.String,
				IsPrimaryKey: Key.String == "PRI",
			})
		}
		rows.Close()
	} else if req.Mode == InitModeSqlite {
		expectTypeMap = map[string]string{
			SqlTypeChar255:  "char(255)",
			SqlTypeLongBlob: "longblob",
			SqlTypeBigInt:   "bigint",
			SqlTypeBool:     "char(255)",
		}
		var rows *sql.Rows
		rows, err = req.Db.Query("pragma table_info(`" + req.TableName + "`)")
		if err != nil {
			return err
		}
		for rows.Next() {
			var idx int
			var notNull int
			var isPrimaryKey int
			var Field sql.NullString
			var Type sql.NullString
			var Default sql.NullString

			err = rows.Scan(&idx, &Field, &Type, &notNull, &Default, &isPrimaryKey)
			if err != nil {
				rows.Close()
				return err
			}
			Type.String = strings.ToLower(Type.String)
			fieldListInDb = append(fieldListInDb, FieldDefInDb{
				Name:         Field.String,
				Type:         Type.String,
				IsPrimaryKey: isPrimaryKey > 0,
			})
		}
		rows.Close()
	} else {
		return errors.New("invalid init Mode " + req.Mode)
	}
	var dropColumnList []string
	var foundFieldList = make([]bool, len(req.FieldList))
	for idxDb, def := range fieldListInDb {
		if len(req.FieldList) > idxDb {
			f := req.FieldList[idxDb]
			if f.IsPrimaryKey != def.IsPrimaryKey {
				panic(`primary key changed1 ` + strconv.Quote(req.TableName) + " " + strconv.Quote(def.Name))
			}
		} else if def.IsPrimaryKey {
			panic(`primary key changed2 ` + strconv.Quote(req.TableName) + " " + strconv.Quote(def.Name))
		}
		found := false
		for idx, f := range req.FieldList {
			if f.Name != def.Name {
				continue
			}
			if f.IsPrimaryKey && idx != idxDb {
				panic(`primary key changed3 ` + strconv.Quote(req.TableName) + " " + strconv.Quote(def.Name))
			}
			found = true
			foundFieldList[idx] = true
			expectedType := expectTypeMap[f.Type]
			if expectedType == "" || def.Type != expectedType {
				return errors.New("field type changed1 " + f.Name + " " + strconv.Quote(expectedType) + " " + strconv.Quote(def.Type))
			}
		}
		if found == false {
			dropColumnList = append(dropColumnList, def.Name)
		}
	}
	var addColumnList []FieldSqlDefine
	for idx, f := range req.FieldList {
		if foundFieldList[idx] {
			continue
		}
		addColumnList = append(addColumnList, f)
	}
	if len(dropColumnList) > 0 && req.Mode == InitModeSqlite {
		fmt.Println("korm skip drop column ", dropColumnList)
		dropColumnList = nil
	}
	if len(addColumnList) > 0 || len(dropColumnList) > 0 {
		buf.Reset()
		buf.WriteString("ALTER TABLE `" + req.TableName + "`")
		for idx, f := range addColumnList {
			var s string
			s, err = getFieldDefine(f)
			if err != nil {
				return err
			}
			if idx > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(" ADD COLUMN " + s)
		}
		if len(addColumnList) > 0 && len(dropColumnList) > 0 {
			buf.WriteString(",")
		}
		for idx, fName := range dropColumnList {
			if idx > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(" DROP COLUMN `" + fName + "`")
		}
		_, err = req.Db.Exec(buf.String())
		if err != nil {
			fmt.Println("korm alter column sql: " + buf.String())
			return err
		}
	}
	if req.Mode == InitModeMysql {
		err = indexMysql_Alter(req.Db, req.TableName, req.IndexList, stringOrLongBlobFieldNameList)
		if err != nil {
			return err
		}
	} else if req.Mode == InitModeSqlite {
		err = indexSqlite_Alter(req.Db, req.TableName, req.IndexList)
		if err != nil {
			return err
		}
	}

	return nil
}
