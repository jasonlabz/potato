package mongo

import (
	"fmt"
	"sync"
)

// RepositoryManager 仓库管理器，管理特定类型的Repository单例
type RepositoryManager struct {
	operator     *MongoOperator
	repositories sync.Map // map[RepositoryKey]*MongoRepository[T]
}

// RepositoryKey 仓库唯一标识
type RepositoryKey struct {
	DBName   string
	CollName string
	TypeName string
}

var (
	repositoryManager     *RepositoryManager
	repositoryManagerOnce sync.Once
)

// GetRepositoryManager 获取仓库管理器单例
func GetRepositoryManager() *RepositoryManager {
	repositoryManagerOnce.Do(func() {
		repositoryManager = &RepositoryManager{
			operator: GetOperator(),
		}
	})
	return repositoryManager
}

// GetRepository 获取特定类型的Repository单例
func GetRepository[T any](dbName, collName string, opts ...RepositoryOption[T]) *MongoRepository[T] {
	manager := GetRepositoryManager()
	key := RepositoryKey{
		DBName:   dbName,
		CollName: collName,
		TypeName: getTypeName[T](),
	}

	// 尝试从缓存获取
	if repo, ok := manager.repositories.Load(key); ok {
		return repo.(*MongoRepository[T])
	}

	// 创建新的Repository（使用双检锁确保线程安全）
	var once sync.Once
	var repository *MongoRepository[T]

	once.Do(func() {
		repository = NewRepository[T](manager.operator, dbName, collName, opts...)
		manager.repositories.Store(key, repository)
	})

	return repository
}

// getTypeName 获取泛型类型的名称
func getTypeName[T any]() string {
	var t T
	return fmt.Sprintf("%T", t)
}

// RemoveRepository 移除特定Repository缓存
func RemoveRepository[T any](dbName, collName string) {
	manager := GetRepositoryManager()
	key := RepositoryKey{
		DBName:   dbName,
		CollName: collName,
		TypeName: getTypeName[T](),
	}
	manager.repositories.Delete(key)
}

// RepositoryBuilder 仓库构建器，支持链式调用
type RepositoryBuilder[T any] struct {
	dbName   string
	collName string
	opts     []RepositoryOption[T]
}

// NewRepositoryBuilder 创建仓库构建器
func NewRepositoryBuilder[T any]() *RepositoryBuilder[T] {
	return &RepositoryBuilder[T]{}
}

// WithDatabase 设置数据库名
func (b *RepositoryBuilder[T]) WithDatabase(dbName string) *RepositoryBuilder[T] {
	b.dbName = dbName
	return b
}

// WithCollection 设置集合名
func (b *RepositoryBuilder[T]) WithCollection(collName string) *RepositoryBuilder[T] {
	b.collName = collName
	return b
}

// WithOptions 设置选项
func (b *RepositoryBuilder[T]) WithOptions(opts ...RepositoryOption[T]) *RepositoryBuilder[T] {
	b.opts = append(b.opts, opts...)
	return b
}

// Build 构建并获取Repository单例
func (b *RepositoryBuilder[T]) Build() *MongoRepository[T] {
	return GetRepository[T](b.dbName, b.collName, b.opts...)
}

// CollectionOps 集合操作封装，简化单例使用
type CollectionOps struct {
	dbName   string
	collName string
}

// NewCollectionOps 创建集合操作实例
func NewCollectionOps(dbName, collName string) *CollectionOps {
	return &CollectionOps{
		dbName:   dbName,
		collName: collName,
	}
}

// ForType 指定文档类型（函数版本，因为方法不能有泛型参数）
func ForType[T any](ops *CollectionOps, opts ...RepositoryOption[T]) *MongoRepository[T] {
	return GetRepository[T](ops.dbName, ops.collName, opts...)
}

// 示例使用方式：
// type User struct {
//     ID   primitive.ObjectID `bson:"_id"`
//     Name string             `bson:"name"`
// }
//
// // 方式1：直接获取单例
// userRepo := GetRepository[User]("mydb", "users")
//
// // 方式2：使用构建器
// userRepo := NewRepositoryBuilder[User]().
//     WithDatabase("mydb").
//     WithCollection("users").
//     Build()
//
// // 方式3：使用集合操作
// ops := NewCollectionOps("mydb", "users")
// userRepo := ops.ForType[User]()
