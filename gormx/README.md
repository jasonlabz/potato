## GORMX 🥔
> gormx支持mysql、postgresql、sqlite、dm等多种关系型数据库的配置读取和初始化等逻辑。

gormx默认会读取conf/application.yaml下的配置：
```yaml
datasource:
  enable: false                # 是否启用
  strict: true                 # 是否为下游必需，如为true则会启动时panic所遇error   
  db_type: "mysql"             # 数据库类型  "mysql|postgres|sqlserver|oracle|sqlite|dm"
  #  dsn: "user:passwd@tcp(*******:8306)/lg_server?charset=utf8mb4&parseTime=True&loc=Local&timeout=20s"
  #  dsn: "user=postgres password=halojeff host=127.0.0.1 port=8432 dbname=lg_server sslmode=disable TimeZone=Asia/Shanghai"
  host: "*******"
  port: 8306
  username: root
  password: "*******"
  database: dbname
  # 主从配置
#  masters:
#    - host: "*******"
#      port: 8306
#      username: root
#      password: "*******"
#      database: dbname
#  replicas:
#    - host: "*******"
#      port: 8306
#      username: root
#      password: "*******"
#      database: dbname
  args:                      # 额外参数    
    - name: charset
      value: utf8mb4
  log_mode: "info"
  max_idle_conn: 10
  max_open_conn: 100
```
- 初始化gorm.DB
```go
// 服务启动时手动读取配置连接数据库，可以选择通过dao.SetGormDB(db)等方式注入gorm.DB实例
func initDB(_ context.Context) {
    dbConf := configx.GetConfig().Database
    if !dbConf.Enable {
        return
    }
    gormConfig := &gormx.Config{}
    err := utils.CopyStruct(dbConf, gormConfig)
    if err != nil {
        panic(err)
    }
    gormConfig.DBName = gormx.DefaultDBNameMaster
    gormConfig.Logger =
        gormx.LoggerAdapter(resource.Logger.WithCallerSkip(3))
    _, err = gormx.InitConfig(gormConfig)
    if err != nil {
        panic(err)
    }
    // dao.SetGormDB(db)
}
```
- 使用gormx
> # 推荐使用过工具生成 dao层代码，如[gentol工具](https://github.com/jasonlabz/gentol)
```go



```