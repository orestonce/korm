package korm

import (
	"bytes"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

func checkIndexListInStruct(info StructType) {
	indexMap := map[string]bool{}

	for _, thisField := range info.FieldList {
		for _, index := range thisField.IndexList {
			foundThisField := false
			indexStr := strings.Join(index, ",")
			if indexMap[indexStr] {
				panic(`index repeated: ` + indexStr + ` ` + strconv.Quote(info.Name))
			}
			indexMap[indexStr] = true

			fieldMap := map[string]bool{}
			for _, fn := range index {
				if fn == thisField.FiledName {
					foundThisField = true
				}
				if fieldMap[fn] { // 字段重复
					panic(`index filed repeated: ` + indexStr + " " + strconv.Quote(info.Name))
				}
				fieldMap[fn] = true

				var foundField *StructFieldType
				for _, f := range info.FieldList {
					if f.FiledName == fn {
						foundField = &f
						if f.IsBoolType() == false && f.IsStringOrIntUintType() == false {
							panic(`index type invalid ` + strconv.Quote(fn) + ` ` + indexStr + ` ` + strconv.Quote(info.Name))
						}
						break
					}
				}
				if foundField == nil {
					panic(`index filed not found: ` + strconv.Quote(fn) + ` ` + indexStr + " " + strconv.Quote(info.Name))
				}
			}
			if foundThisField == false {
				//只能定义和本字段相关的index
				panic(`index ` + indexStr + " " + strconv.Quote(info.Name) + ` is not related to field ` + strconv.Quote(thisField.FiledName))
			}
		}
	}
}

func indexListToDeclString(info StructType) string {
	var indexList [][]string

	for _, field := range info.FieldList {
		for _, index := range field.IndexList {
			indexList = append(indexList, index)
		}
	}
	if len(indexList) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	buf.WriteString("IndexList: [][]string{\n")
	for _, index := range indexList {
		buf.WriteString("{")
		for _, fn := range index {
			buf.WriteString(strconv.Quote(fn) + `,`)
		}
		buf.WriteString("},\n")
	}
	buf.WriteString("},\n")
	return buf.String()
}

func indexInCreateSql(buf *bytes.Buffer, tableName string, indexList [][]string, stringOrLongBlobFieldNameList []string) {
	if len(indexList) == 0 {
		return
	}
	for idx, index := range indexList {
		buf.WriteString(`, index ` + indexKeyNamePrefix + strconv.Itoa(idx) + "_" + tableName + `(`)
		for fnIdx, fn := range index {
			if fnIdx > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("`" + fn + "`")
			if isInStringList(fn, stringOrLongBlobFieldNameList) {
				buf.WriteString(`(255)`)
			}
		}
		buf.WriteString(`)`)
	}
}

func isInStringList(fn string, ss []string) bool {
	for _, fnStr := range ss {
		if fnStr == fn {
			return true
		}
	}
	return false
}

type indexItem struct {
	Name          string
	FieldNameList []string
}

func indexMysql_Alter(db *sql.DB, tableName string, expectIndexList [][]string, stringOrLongBlobFieldNameList []string) (err error) {
	indexMapInDb, err := indexGetMysql(db, tableName)
	if err != nil {
		return err
	}
	dropIndexList, addIndexItem := indexDiff(tableName, indexMapInDb, expectIndexList)
	if len(dropIndexList) == 0 && len(addIndexItem) == 0 {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	buf.WriteString("ALTER TABLE `" + tableName + "` ")
	for idx, name := range dropIndexList {
		if idx > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("DROP INDEX `" + name + "`")
	}
	if len(dropIndexList) > 0 && len(addIndexItem) > 0 {
		buf.WriteString(", ")
	}
	for idx, item := range addIndexItem {
		if idx > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString("ADD INDEX `" + item.Name + "`(")
		for idxFn, fn := range item.FieldNameList {
			if idxFn > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("`" + fn + "`")
			if isInStringList(fn, stringOrLongBlobFieldNameList) {
				buf.WriteString(`(255)`)
			}
		}
		buf.WriteString(")")
	}
	_, err = db.Exec(buf.String())
	if err != nil {
		fmt.Println("korm ", buf.String())
		return err
	}
	return nil
}

func isStringListEq(s1 []string, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for idx := 0; idx < len(s1); idx++ {
		if s1[idx] != s2[idx] {
			return false
		}
	}
	return true
}

func indexGetMysql(db *sql.DB, tableName string) (m map[string]indexItem, err error) {
	var rows *sql.Rows
	rows, err = db.Query("SHOW INDEX FROM `" + tableName + "`")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m = map[string]indexItem{}
	for rows.Next() {
		var Table string
		var Non_unique int
		var Key_name string
		var Seq_in_index int
		var Column_name string
		var Collation string
		var Cardinality string
		var Sub_part sql.NullString
		var Packed sql.NullString
		var Null sql.NullString
		var Index_type sql.NullString
		var Comment sql.NullString
		var Index_comment sql.NullString
		var Visible sql.NullString
		var Expression sql.NullString

		err = rows.Scan(&Table, &Non_unique, &Key_name, &Seq_in_index, &Column_name, &Collation, &Cardinality, &Sub_part, &Packed, &Null, &Index_type, &Comment, &Index_comment, &Visible, &Expression)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(Key_name, indexKeyNamePrefix) == false {
			continue
		}
		item := m[Key_name]
		item.Name = Key_name
		item.FieldNameList = append(item.FieldNameList, Column_name)
		m[item.Name] = item
	}
	return m, nil
}

const indexKeyNamePrefix = `korm_idx_`

func indexGetSqlite(db *sql.DB, tableName string) (m map[string]indexItem, err error) {
	rows, err := db.Query("SELECT * FROM sqlite_master WHERE type = 'index' and tbl_name = ?", tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m = map[string]indexItem{}
	for rows.Next() {
		var c_type sql.NullString
		var c_name string
		var c_tbl_name sql.NullString
		var c_rootpage sql.NullString
		var c_sql sql.NullString
		err = rows.Scan(&c_type, &c_name, &c_tbl_name, &c_rootpage, &c_sql)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(c_name, indexKeyNamePrefix) == false {
			continue
		}
		m[c_name] = indexItem{
			Name:          c_name,
			FieldNameList: indexSqliteParse(c_sql.String),
		}
	}
	return m, nil
}
func indexSqlite_Alter(db *sql.DB, tableName string, expectIndexList [][]string) (err error) {
	indexMapInDb, err := indexGetSqlite(db, tableName)
	if err != nil {
		return err
	}
	dropIndexList, addIndexItem := indexDiff(tableName, indexMapInDb, expectIndexList)
	if len(dropIndexList) == 0 && len(addIndexItem) == 0 {
		return nil
	}
	for _, name := range dropIndexList {
		_, err = db.Exec("DROP INDEX `" + name + "`")
		if err != nil {
			return err
		}
	}
	for _, item := range addIndexItem {
		buf := bytes.NewBuffer(nil)
		buf.WriteString("CREATE INDEX `" + item.Name + "` ON `" + tableName + "`(")
		for idx, name := range item.FieldNameList {
			if idx > 0 {
				buf.WriteString(",")
			}
			buf.WriteString("`" + name + "`")
		}
		buf.WriteString(")")
		_, err = db.Exec(buf.String())
		if err != nil {
			fmt.Println(buf.String())
			return err
		}
	}
	return nil
}

func indexSqliteParse(c_sql string) (list []string) {
	sqlStr := strings.TrimSpace(c_sql)
	idx0 := strings.LastIndex(sqlStr, `(`)
	idx1 := strings.LastIndex(sqlStr, `)`)
	if idx0 < 0 || idx1 < 0 || idx0 > idx1 {
		return nil
	}
	fieldList := strings.Split(strings.TrimSpace(sqlStr[idx0+1:idx1]), `,`)
	for _, field := range fieldList {
		field = strings.TrimSpace(field)
		field = strings.Trim(field, "`")
		list = append(list, field)
	}
	return list
}

func indexDiff(tableName string, indexMapInDb map[string]indexItem, expectIndexList [][]string) (dropIndexList []string, addIndexItem []indexItem) {
	if len(indexMapInDb) == 0 && len(expectIndexList) == 0 {
		return nil, nil
	}

	indexMapExpect := map[string]indexItem{}
	for _, index := range expectIndexList {
		for i := 0; ; i++ {
			if i > 10000 {
				panic(`indexMysql_Alter not found index name for ` + strconv.Quote(tableName))
			}
			name := indexKeyNamePrefix + strconv.Itoa(i) + "_" + tableName
			_, ok1 := indexMapInDb[name]
			_, ok2 := indexMapExpect[name]
			if ok1 == false && ok2 == false {
				indexMapExpect[name] = indexItem{
					Name:          name,
					FieldNameList: index,
				}
				break
			}
		}
	}

	for _, inDb := range indexMapInDb {
		for _, inExpect := range indexMapExpect {
			if isStringListEq(inDb.FieldNameList, inExpect.FieldNameList) {
				delete(indexMapInDb, inDb.Name)
				delete(indexMapExpect, inExpect.Name)
				break
			}
		}
	}
	// 删除 indexMapInDb, 添加indexMapExpect
	if len(indexMapInDb) == 0 && len(indexMapExpect) == 0 {
		return nil, nil
	}
	for name := range indexMapInDb {
		dropIndexList = append(dropIndexList, name)
	}
	for _, item := range indexMapExpect {
		addIndexItem = append(addIndexItem, item)
	}

	return dropIndexList, addIndexItem
}
