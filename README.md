# korm

# 特点:
1. 从struct结构定义数据表结构，自动裁剪/增加数据表里的字段
2. 强类型,代码提示友好。
````golang
    afected = db.test01Crud_D().Update().Where_Id().Equal(10).Set_Name("10").MustRun()
    value, ok = db.test01Crud_D().Select().Where_Name().Equal("10").MustRun_ResultOne2()
````
3. 支持Left Join抽象。支持任意路径的LeftJoin方式
4. 支持view抽象
5. 支持生成Must接口、error接口
6. Select查询时，支持强类型返回值:

| 目标                | 函数名                              |
|-------------------|----------------------------------|
| 返回是否存在            | MustRun_Exists                   |
| 返回匹配的个数           | MustRun_Count                    |
| 返回一个值             | MustRun_ResultOne                |
| 返回一个列表            | MustRun_ResultList               |
| 返回一个列表，并且返回总的匹配个数 | MustRun_ResultListWithTotalMatch |
| 返回一个以主键为key的Map   | MustRun_ResultMap                |