# mongo

泛型 MongoDB Repository 封装，提供类型安全的 CRUD、聚合、分页、事务和 Change Stream 支持。

## 核心类型

- `MongoOperator` - MongoDB 连接管理器
- `MongoRepository[T any]` - 泛型 Repository，绑定到特定集合

## 使用示例

### 初始化

```go
import "github.com/jasonlabz/potato/mongo"

err := mongo.InitMongoOperator(ctx, &mongo.Config{
    URI:      "mongodb://localhost:27017",
    Database: "mydb",
})
op := mongo.GetOperator()
```

### 创建 Repository

```go
type User struct {
    ID   primitive.ObjectID `bson:"_id,omitempty"`
    Name string             `bson:"name"`
    Age  int                `bson:"age"`
}

repo := mongo.NewRepository[User](op, "mydb", "users")
```

### CRUD 操作

```go
// 插入
id, err := repo.InsertOne(ctx, &User{Name: "张三", Age: 30})

// 查询
user, err := repo.FindByID(ctx, id)
users, err := repo.Find(ctx, bson.M{"age": bson.M{"$gt": 18}})

// 更新
result, err := repo.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"name": "李四"}})

// 删除
result, err := repo.DeleteOne(ctx, bson.M{"_id": id})
```

### 分页查询

```go
users, total, err := repo.Paginate(ctx, bson.M{}, 1, 20)
```

### 聚合查询

```go
pipeline := bson.A{
    bson.M{"$group": bson.M{"_id": "$age", "count": bson.M{"$sum": 1}}},
}
results, err := repo.Aggregate(ctx, pipeline)
```

### 事务

```go
err := repo.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
    _, err := repo.InsertOne(sessCtx, &user1)
    _, err = repo.InsertOne(sessCtx, &user2)
    return err
})
```

### Change Stream

```go
stream, err := repo.Watch(ctx, bson.A{})
defer stream.Close(ctx)
for stream.Next(ctx) {
    // 处理变更事件
}
```

### 批量写入

```go
models := []mongodriver.WriteModel{
    mongodriver.NewInsertOneModel().SetDocument(&user),
    mongodriver.NewUpdateOneModel().SetFilter(filter).SetUpdate(update),
}
result, err := repo.BulkWrite(ctx, models)
```
