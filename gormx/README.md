# gormx

GORM 数据库封装，支持多种数据库类型和读写分离，提供统一的连接管理。

## 支持的数据库

- MySQL
- PostgreSQL
- SQL Server
- Oracle
- SQLite
- DM（达梦）

## 默认连接池参数

| 参数 | 默认值 |
|------|--------|
| MaxOpenConn | 100 |
| MaxIdleConn | 10 |
| ConnMaxLifeTime | 3 小时 |
| ConnMaxIdleTime | 45 分钟 |

## 配置文件

默认读取 `conf/application.yaml`：

```yaml
datasource:
  enable: false                # 是否启用
  strict: true                 # 是否为下游必需，如为 true 则启动时 panic
  db_type: "mysql"             # mysql|postgres|sqlserver|oracle|sqlite|dm
  host: "127.0.0.1"
  port: 3306
  username: root
  password: "xxx"
  database: mydb
  # 或者直接使用 DSN
  # dsn: "root:pass@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local"
  # 主从配置
  # masters:
  #   - host: "127.0.0.1"
  #     port: 3306
  #     username: root
  #     password: "xxx"
  #     database: mydb
  # replicas:
  #   - host: "127.0.0.1"
  #     port: 3307
  #     username: readonly
  #     password: "xxx"
  #     database: mydb
  args:
    - name: charset
      value: utf8mb4
  log_mode: "info"
  max_idle_conn: 10
  max_open_conn: 100
```

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/gormx"

// 从全局配置初始化
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
    _, err = gormx.InitConfig(gormConfig)
    if err != nil {
        panic(err)
    }
}
```

### 获取数据库实例

```go
// 按名称获取
db := gormx.GetDB("mydb")

// 获取默认主库/从库
masterDB := gormx.DefaultMaster()
slaveDB := gormx.DefaultSlave()

// 连接检查
err := gormx.Ping("mydb")

// 关闭连接
gormx.Close("mydb")
```

> 推荐使用 [gentol](https://github.com/jasonlabz/gentol) 工具自动生成 DAO 层代码。

## 特性

- 主从读写分离（基于 gorm dbresolver 插件）
- 连接池管理
- Mock 模式支持测试
- 自动 DSN 生成（从 Host/Port/User/Pass 构建）
- 支持 DSN 直连或参数拆分两种配置方式
