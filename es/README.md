# es

Elasticsearch v9 客户端封装，提供连接管理和常用 CRUD、搜索、批量操作。

## 核心类型

- `ESOperator` - Elasticsearch 操作实例，管理 ES 客户端连接
- `Config` - 连接配置，支持 Basic Auth、API Key、Cloud ID、HTTPS

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/es"

// 通过配置文件自动初始化
// 或手动初始化
operator, err := es.NewESOperator(&es.Config{
    Addresses: []string{"http://localhost:9200"},
    Username:  "elastic",
    Password:  "password",
})
```

### 配置集成

```yaml
es:
  addresses:
    - "http://localhost:9200"
  username: "elastic"
  password: "password"
```

## 支持的认证方式

- Basic Auth（用户名/密码）
- API Key
- Cloud ID
- HTTPS（自定义 CA 证书）

## 依赖

- `github.com/elastic/go-elasticsearch/v9`
