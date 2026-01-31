package mongo

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CollectionManager 集合管理器，实现单例集合操作
type CollectionManager struct {
	operator    *MongoOperator
	collections sync.Map // concurrent map for thread-safe collection access
}

var (
	collectionManager     *CollectionManager
	collectionManagerOnce sync.Once
)

// GetCollectionManager 获取集合管理器单例
func GetCollectionManager() *CollectionManager {
	collectionManagerOnce.Do(func() {
		collectionManager = &CollectionManager{
			operator: GetOperator(),
		}
	})
	return collectionManager
}

// GetCollection 获取指定集合的单例实例
func (cm *CollectionManager) GetCollection(dbName, collName string) *mongo.Collection {
	key := dbName + "." + collName

	// 尝试从缓存中获取
	if coll, ok := cm.collections.Load(key); ok {
		return coll.(*mongo.Collection)
	}

	// 创建新的集合实例
	coll := cm.operator.GetClient().Database(dbName).Collection(collName)

	// 存储到缓存
	cm.collections.Store(key, coll)

	return coll
}

// RemoveCollection 移除集合缓存
func (cm *CollectionManager) RemoveCollection(dbName, collName string) {
	key := dbName + "." + collName
	cm.collections.Delete(key)
}

// ClearAll 清空所有集合缓存
func (cm *CollectionManager) ClearAll() {
	cm.collections.Range(func(key, value any) bool {
		cm.collections.Delete(key)
		return true
	})
}

// CollectionStats 获取集合统计信息
func (cm *CollectionManager) CollectionStats(ctx context.Context, dbName, collName string) (bson.M, error) {
	coll := cm.GetCollection(dbName, collName)

	command := bson.D{{Key: "collStats", Value: collName}}
	var result bson.M
	err := coll.Database().RunCommand(ctx, command).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
