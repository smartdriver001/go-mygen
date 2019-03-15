package logic

import (
	"bytes"
	"strings"
	"text/template"

	"fmt"

	"github.com/yezihack/colorlog"
	"github.com/yezihack/gm2m/common"
	"github.com/yezihack/gm2m/conf"
	"github.com/yezihack/gm2m/mysql"
	tpldata "github.com/yezihack/gm2m/tpl"
)

type Logic struct {
	T  *common.Tools
	DB *mysql.ModelS
}

//创建和获取MYSQL目录
func (l *Logic) GetMysqlDir() string {
	return CreateDir(common.GetExeRootDir() + conf.GODIR_MODELS + conf.DS)
}

//获取根目录地址
func (l *Logic) GetRoot() string {
	return common.GetRootPath(common.GetExeRootDir()) + conf.DS
}

//创建结构实体
func (l *Logic) GenerateDBEntity(req *mysql.EntityReq) (err error) {
	var s string
	s = fmt.Sprintf(`//判断package是否加载过
package %s
import (
	"database/sql"
	"github.com/go-sql-driver/mysql"
)
`, req.Pkg)
	//判断import是否加载过
	check := "github.com/go-sql-driver/mysql"
	if l.T.CheckFileContainsChar(req.Path, check) == false {
		l.T.WriteFile(req.Path, s)
	}
	//声明表结构变量
	TableData := new(mysql.TableInfo)
	TableData.Table = l.T.Capitalize(req.TableName)
	TableData.NullTable = TableData.Table + conf.DbNullPrefix
	TableData.TableComment = req.TableComment
	//判断表结构是否加载过
	if l.T.CheckFileContainsChar(req.Path, "type "+TableData.Table+" struct") == true {
		return
	}
	//加载模板文件
	tplByte, err := tpldata.Asset(conf.TPL_ENTITY)
	if err != nil {
		return
	}
	tpl, err := template.New("entity").Parse(string(tplByte))
	if err != nil {
		colorlog.Error("ParseFiles", err)
		return
	}
	//装载表字段信息
	for _, val := range req.TableDesc {
		TableData.Fields = append(TableData.Fields, &mysql.FieldsInfo{
			Name:         l.T.Capitalize(val.ColumnName),
			Type:         val.GolangType,
			NullType:     val.MysqlNullType,
			DbOriField:   val.ColumnName,
			FormatFields: common.FormatField(val.ColumnName, req.FormatList),
			Remark:       val.ColumnComment,
		})
	}
	content := bytes.NewBuffer([]byte{})
	tpl.Execute(content, TableData)
	//表信息写入文件

	err = WriteAppendFile(req.Path, content.String())
	if err != nil {
		return
	}
	return
}

//生成C增,U删,R查,D改,的文件
func (l *Logic) GenerateCURDFile(tableName, tableComment string, tableDesc []*mysql.TableDesc) (err error) {
	allFields := make([]string, 0)
	insertFields := make([]string, 0)
	InsertInfo := make([]*mysql.SqlFieldInfo, 0)
	fieldsList := make([]*mysql.SqlFieldInfo, 0)
	nullFieldList := make([]*mysql.NullSqlFieldInfo, 0)
	updateList := make([]string, 0)
	updateListField := make([]string, 0)
	PrimaryKey, primaryType := "", ""
	for _, item := range tableDesc {
		allFields = append(allFields, "`"+item.ColumnName+"`")
		if item.PrimaryKey == false && item.ColumnName != "updated_at" && item.ColumnName != "created_at" {
			insertFields = append(insertFields, item.ColumnName)
			InsertInfo = append(InsertInfo, &mysql.SqlFieldInfo{
				HumpName: l.T.Capitalize(item.ColumnName),
				Comment:  item.ColumnComment,
			})
			if item.ColumnName == "identify" {
				updateList = append(updateList, item.ColumnName+"="+item.ColumnName+"+1")
			} else {
				updateList = append(updateList, item.ColumnName+"=?")
				if item.PrimaryKey == false {
					updateListField = append(updateListField, "value."+l.T.Capitalize(item.ColumnName))
				}
			}
		}
		if item.PrimaryKey {
			PrimaryKey = item.ColumnName
			primaryType = item.GolangType
		}
		fieldsList = append(fieldsList, &mysql.SqlFieldInfo{
			HumpName: l.T.Capitalize(item.ColumnName),
			Comment:  item.ColumnComment,
		})
		nullFieldList = append(nullFieldList, &mysql.NullSqlFieldInfo{
			HumpName:     l.T.Capitalize(item.ColumnName),
			OriFieldType: item.OriMysqlType,
			GoType:       conf.MysqlTypeToGoType[item.OriMysqlType],
			Comment:      item.ColumnComment,
		})
	}
	//主键ID,用于更新
	if PrimaryKey != "" {
		updateListField = append(updateListField, "value."+l.T.Capitalize(PrimaryKey))
	}
	//拼出SQL所需要结构数据
	InsertMark := strings.Repeat("?,", len(insertFields))
	sqlInfo := &mysql.SqlInfo{
		TableName:           tableName,
		PrimaryKey:          PrimaryKey,
		PrimaryType:         primaryType,
		StructTableName:     l.T.Capitalize(tableName),
		NullStructTableName: l.T.Capitalize(tableName) + conf.DbNullPrefix,
		UpperTableName:      conf.TablePrefix + l.T.ToUpper(tableName),
		AllFieldList:        strings.Join(allFields, ","),
		InsertFieldList:     strings.Join(insertFields, ","),
		InsertMark:          InsertMark[:len(InsertMark)-1],
		UpdateFieldList:     strings.Join(updateList, ","),
		UpdateListField:     updateListField,
		FieldsInfo:          fieldsList,
		NullFieldsInfo:      nullFieldList,
		InsertInfo:          InsertInfo,
	}
	err = l.GenerateSQL(sqlInfo, tableComment)
	if err != nil {
		return
	}
	return
}

//生成表列表
func (l *Logic) GenerateTableList(list []*mysql.TableList) (err error) {
	//写入表名
	tableListFile := l.GetMysqlDir() + conf.GoFile_TableList
	//判断package是否加载过
	checkStr := "package " + conf.PkgDbModels
	if l.T.CheckFileContainsChar(tableListFile, checkStr) == false {
		l.T.WriteFile(tableListFile, checkStr+"\n")
	}
	checkStr = "const"
	if l.T.CheckFileContainsChar(tableListFile, checkStr) {
		return
	}
	tplByte, err := tpldata.Asset(conf.TPL_TABLES)
	if err != nil {
		return
	}
	tpl, err := template.New("table_list").Parse(string(tplByte))
	if err != nil {
		return
	}
	//解析
	content := bytes.NewBuffer([]byte{})
	err = tpl.Execute(content, list)
	if err != nil {
		return
	}
	//表信息写入文件
	err = WriteAppendFile(tableListFile, content.String())
	if err != nil {
		return
	}
	return
}

//生成SQL文件
func (l *Logic) GenerateSQL(info *mysql.SqlInfo, tableComment string) (err error) {
	//写入表名
	goFile := l.GetMysqlDir() + info.TableName + ".go"
	s := fmt.Sprintf(`
//%s
package %s
import(
	"database/sql"
)
`, tableComment, conf.PkgDbModels)
	//判断package是否加载过
	if l.T.CheckFileContainsChar(goFile, "database/sql") == false {
		l.T.WriteFile(goFile, s)
	}

	//解析模板
	tplByte, err := tpldata.Asset(conf.TPL_CURD)
	if err != nil {
		return
	}
	tpl, err := template.New("CURD").Parse(string(tplByte))
	if err != nil {
		return
	}
	//解析
	content := bytes.NewBuffer([]byte{})
	err = tpl.Execute(content, info)
	if err != nil {
		return
	}
	//表信息写入文件
	if l.T.CheckFileContainsChar(goFile, info.StructTableName) == false {
		err = WriteAppendFile(goFile, content.String())
		if err != nil {
			return
		}
	}
	return
}

//生成表列表
func (l *Logic) GenerateMarkdown(data *mysql.MarkDownData) (err error) {
	//写入markdown
	file := common.GetExeRootDir() + "markdown.md"
	tplByte, err := tpldata.Asset(conf.TPL_MARKDOWN)
	if err != nil {
		return
	}
	//解析
	content := bytes.NewBuffer([]byte{})
	tpl, err := template.New("markdown").Parse(string(tplByte))
	err = tpl.Execute(content, data)
	if err != nil {
		return
	}
	//表信息写入文件
	err = WriteAppendFile(file, content.String())
	if err != nil {
		return
	}
	return
}
