// Package mongo
//
//   _ __ ___   __ _ _ __  _   _| |_
//  | '_ ` _ \ / _` | '_ \| | | | __|
//  | | | | | | (_| | | | | |_| | |_
//  |_| |_| |_|\__,_|_| |_|\__,_|\__|
//
//  Buddha bless, no bugs forever!
//
//  Author:    lucas
//  Email:     1783022886@qq.com
//  Created:   2026/1/24 15:40
//  Version:   v2.0.0

package mongo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/jasonlabz/potato/log"
)

var (
	operator     *MongoOperator
	operatorOnce sync.Once
	operatorMu   sync.RWMutex

	// 默认配置
	defaultConfig = &Config{
		MaxPoolSize:            100,
		MinPoolSize:            10,
		MaxConnIdleTime:        30,
		ConnectTimeout:         10,
		SocketTimeout:          30,
		ServerSelectionTimeout: 10,
		HeartbeatInterval:      10,
		ReplicaSet:             "",
	}
)

type MongoOperator struct {
	cli    *mongo.Client
	config *Config
	l      *log.LoggerWrapper
}

// GetOperator 获取全局操作器
func GetOperator() *MongoOperator {
	if operator == nil {
		panic("mongo operator not initialized, call InitMongoOperator first")
	}
	return operator
}

// GetClient 获取原始客户端
func (m *MongoOperator) GetClient() *mongo.Client {
	return m.cli
}

// Ping 检查连接
func (m *MongoOperator) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.cli.Ping(ctx, readpref.Primary())
}

// Close 关闭连接
func (m *MongoOperator) Close(ctx context.Context) error {
	if m.cli != nil {
		return m.cli.Disconnect(ctx)
	}
	return nil
}

// InitMongoOperator 初始化全局操作器
func InitMongoOperator(ctx context.Context, config *Config) (err error) {
	operatorOnce.Do(func() {
		operatorMu.Lock()
		defer operatorMu.Unlock()

		operator, err = NewMongoOperator(ctx, config)
		if err != nil {
			return
		}

		// 测试连接
		if err = operator.Ping(ctx); err != nil {
			operator = nil
			return
		}
	})
	return err
}

// NewMongoOperator 创建新的操作器
func NewMongoOperator(ctx context.Context, config *Config) (*MongoOperator, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}

	// 设置默认配置
	config = applyDefaultConfig(config)

	// 验证必要参数
	if config.URI == "" {
		return nil, errors.New("mongo URI is required")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(config.ConnectTimeout)*time.Second)
	defer cancel()

	// 构建客户端选项
	clientOpts := buildClientOptions(config)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, errors.Wrap(err, "connect to mongo failed")
	}

	// 创建操作器
	op := &MongoOperator{
		config: config,
		cli:    client,
	}

	// 设置日志器
	if config.logger == nil {
		config.logger = log.GetLogger()
	}
	op.l = config.logger

	op.l.Info(ctx, "MongoDB operator initialized successfully",
		"uri", maskURI(config.URI),
		"maxPoolSize", config.MaxPoolSize,
		"minPoolSize", config.MinPoolSize)

	return op, nil
}

type MongoRepository[T any] struct {
	client     *mongo.Client
	dbName     string
	collName   string
	collection *mongo.Collection
	logger     *log.LoggerWrapper
}

// NewRepository 创建新的仓库
func NewRepository[T any](client *mongo.Client, dbName, collName string, opts ...RepositoryOption[T]) *MongoRepository[T] {
	repo := &MongoRepository[T]{
		client:     client,
		dbName:     dbName,
		collName:   collName,
		collection: client.Database(dbName).Collection(collName),
		logger:     log.GetLogger(),
	}

	for _, opt := range opts {
		opt(repo)
	}

	return repo
}

// RepositoryOption 仓库选项
type RepositoryOption[T any] func(*MongoRepository[T])

// WithLogger 设置日志器
func WithLogger[T any](logger *log.LoggerWrapper) RepositoryOption[T] {
	return func(r *MongoRepository[T]) {
		r.logger = logger
	}
}

// WithCollectionOptions 设置集合选项
func WithCollectionOptions[T any](opts ...*options.CollectionOptions) RepositoryOption[T] {
	return func(r *MongoRepository[T]) {
		r.collection = r.client.Database(r.dbName).Collection(r.collName, opts...)
	}
}

// ==================== CRUD 操作 ====================

// InsertOne 插入单个文档
func (r *MongoRepository[T]) InsertOne(ctx context.Context, doc *T, opts ...*options.InsertOneOptions) (primitive.ObjectID, error) {
	start := time.Now()
	defer func() {
		r.logger.Debug(ctx, "mongo insert one completed",
			"collection", r.collName,
			"duration", time.Since(start))
	}()

	result, err := r.collection.InsertOne(ctx, doc, opts...)
	if err != nil {
		return primitive.NilObjectID, errors.Wrap(err, "insert failed")
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		return id, nil
	}

	// 尝试从 interface{} 转换
	switch v := result.InsertedID.(type) {
	case string:
		return primitive.ObjectIDFromHex(v)
	case []byte:
		return primitive.ObjectIDFromHex(string(v))
	default:
		return primitive.NilObjectID, errors.New("invalid inserted ID type")
	}
}

// InsertMany 批量插入文档
func (r *MongoRepository[T]) InsertMany(ctx context.Context, docs []any, opts ...*options.InsertManyOptions) ([]primitive.ObjectID, error) {
	start := time.Now()
	defer func() {
		r.logger.Debug(ctx, "mongo insert many completed",
			"collection", r.collName,
			"count", len(docs),
			"duration", time.Since(start))
	}()

	result, err := r.collection.InsertMany(ctx, docs, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "insert many failed")
	}

	ids := make([]primitive.ObjectID, 0, len(result.InsertedIDs))
	for _, id := range result.InsertedIDs {
		if oid, ok := id.(primitive.ObjectID); ok {
			ids = append(ids, oid)
		}
	}

	return ids, nil
}

// FindByID 根据ID查找
func (r *MongoRepository[T]) FindByID(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (*T, error) {
	return r.FindOne(ctx, bson.M{"_id": id}, opts...)
}

// FindOne 查找单个文档
func (r *MongoRepository[T]) FindOne(ctx context.Context, filter any, opts ...*options.FindOneOptions) (*T, error) {
	var result T
	err := r.collection.FindOne(ctx, filter, opts...).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errors.Wrap(err, "find one failed")
	}
	return &result, nil
}

// Find 查询多个文档
func (r *MongoRepository[T]) Find(ctx context.Context, filter any, opts ...*options.FindOptions) ([]T, error) {
	start := time.Now()
	defer func() {
		r.logger.Debug(ctx, "mongo find completed",
			"collection", r.collName,
			"duration", time.Since(start))
	}()

	cursor, err := r.collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "find query failed")
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decode results failed")
	}
	return results, nil
}

// Count 统计文档数量
func (r *MongoRepository[T]) Count(ctx context.Context, filter any, opts ...*options.CountOptions) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, filter, opts...)
	if err != nil {
		return 0, errors.Wrap(err, "count documents failed")
	}
	return count, nil
}

// EstimatedCount 估计文档数量（更快，但不准确）
func (r *MongoRepository[T]) EstimatedCount(ctx context.Context, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	count, err := r.collection.EstimatedDocumentCount(ctx, opts...)
	if err != nil {
		return 0, errors.Wrap(err, "estimated document count failed")
	}
	return count, nil
}

// UpdateByID 根据ID更新
func (r *MongoRepository[T]) UpdateByID(ctx context.Context, id primitive.ObjectID, update any, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	return r.UpdateOne(ctx, bson.M{"_id": id}, update, opts...)
}

// UpdateOne 更新单个文档
func (r *MongoRepository[T]) UpdateOne(ctx context.Context, filter, update any, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	result, err := r.collection.UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "update one failed")
	}
	return result, nil
}

// UpdateMany 更新多个文档
func (r *MongoRepository[T]) UpdateMany(ctx context.Context, filter, update any, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	result, err := r.collection.UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "update many failed")
	}
	return result, nil
}

// ReplaceOne 替换单个文档
func (r *MongoRepository[T]) ReplaceOne(ctx context.Context, filter any, replacement any, opts ...*options.ReplaceOptions) (*mongo.UpdateResult, error) {
	result, err := r.collection.ReplaceOne(ctx, filter, replacement, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "replace one failed")
	}
	return result, nil
}

// DeleteByID 根据ID删除
func (r *MongoRepository[T]) DeleteByID(ctx context.Context, id primitive.ObjectID, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	return r.DeleteOne(ctx, bson.M{"_id": id}, opts...)
}

// DeleteOne 删除单个文档
func (r *MongoRepository[T]) DeleteOne(ctx context.Context, filter any, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	result, err := r.collection.DeleteOne(ctx, filter, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "delete one failed")
	}
	return result, nil
}

// DeleteMany 删除多个文档
func (r *MongoRepository[T]) DeleteMany(ctx context.Context, filter any, opts ...*options.DeleteOptions) (*mongo.DeleteResult, error) {
	result, err := r.collection.DeleteMany(ctx, filter, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "delete many failed")
	}
	return result, nil
}

// ==================== 高级操作 ====================

// FindOneAndUpdate 查找并更新
func (r *MongoRepository[T]) FindOneAndUpdate(ctx context.Context, filter, update any, opts ...*options.FindOneAndUpdateOptions) (*T, error) {
	var result T
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts...).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errors.Wrap(err, "find one and update failed")
	}
	return &result, nil
}

// FindOneAndReplace 查找并替换
func (r *MongoRepository[T]) FindOneAndReplace(ctx context.Context, filter, replacement any, opts ...*options.FindOneAndReplaceOptions) (*T, error) {
	var result T
	err := r.collection.FindOneAndReplace(ctx, filter, replacement, opts...).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errors.Wrap(err, "find one and replace failed")
	}
	return &result, nil
}

// FindOneAndDelete 查找并删除
func (r *MongoRepository[T]) FindOneAndDelete(ctx context.Context, filter any, opts ...*options.FindOneAndDeleteOptions) (*T, error) {
	var result T
	err := r.collection.FindOneAndDelete(ctx, filter, opts...).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, errors.Wrap(err, "find one and delete failed")
	}
	return &result, nil
}

// BulkWrite 批量写操作
func (r *MongoRepository[T]) BulkWrite(ctx context.Context, models []mongo.WriteModel, opts ...*options.BulkWriteOptions) (*mongo.BulkWriteResult, error) {
	result, err := r.collection.BulkWrite(ctx, models, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "bulk write failed")
	}
	return result, nil
}

// Distinct 去重查询
func (r *MongoRepository[T]) Distinct(ctx context.Context, fieldName string, filter any, opts ...*options.DistinctOptions) ([]any, error) {
	values, err := r.collection.Distinct(ctx, fieldName, filter, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "distinct query failed")
	}
	return values, nil
}

// Aggregate 聚合查询
func (r *MongoRepository[T]) Aggregate(ctx context.Context, pipeline any, opts ...*options.AggregateOptions) ([]T, error) {
	start := time.Now()
	defer func() {
		r.logger.Debug(ctx, "mongo aggregate completed",
			"collection", r.collName,
			"duration", time.Since(start))
	}()

	cursor, err := r.collection.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "aggregate query failed")
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decode aggregate results failed")
	}
	return results, nil
}

// AggregateRaw 原始聚合查询（返回 bson.Raw）
func (r *MongoRepository[T]) AggregateRaw(ctx context.Context, pipeline any, opts ...*options.AggregateOptions) ([]bson.Raw, error) {
	cursor, err := r.collection.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "aggregate query failed")
	}
	defer cursor.Close(ctx)

	var results []bson.Raw
	if err = cursor.All(ctx, &results); err != nil {
		return nil, errors.Wrap(err, "decode raw results failed")
	}
	return results, nil
}

// ==================== 索引操作 ====================

// CreateIndex 创建索引
func (r *MongoRepository[T]) CreateIndex(ctx context.Context, model mongo.IndexModel, opts ...*options.CreateIndexesOptions) (string, error) {
	name, err := r.collection.Indexes().CreateOne(ctx, model, opts...)
	if err != nil {
		return "", errors.Wrap(err, "create index failed")
	}
	return name, nil
}

// CreateIndexes 创建多个索引
func (r *MongoRepository[T]) CreateIndexes(ctx context.Context, models []mongo.IndexModel, opts ...*options.CreateIndexesOptions) ([]string, error) {
	names, err := r.collection.Indexes().CreateMany(ctx, models, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "create indexes failed")
	}
	return names, nil
}

// DropIndex 删除索引
func (r *MongoRepository[T]) DropIndex(ctx context.Context, name string, opts ...*options.DropIndexesOptions) error {
	_, err := r.collection.Indexes().DropOne(ctx, name, opts...)
	if err != nil {
		return errors.Wrap(err, "drop index failed")
	}
	return nil
}

// DropAllIndexes 删除所有索引
func (r *MongoRepository[T]) DropAllIndexes(ctx context.Context, opts ...*options.DropIndexesOptions) error {
	_, err := r.collection.Indexes().DropAll(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "drop all indexes failed")
	}
	return nil
}

// ListIndexes 列出索引
func (r *MongoRepository[T]) ListIndexes(ctx context.Context, opts ...*options.ListIndexesOptions) (*mongo.Cursor, error) {
	cursor, err := r.collection.Indexes().List(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "list indexes failed")
	}
	return cursor, nil
}

// ==================== 事务操作 ====================

// WithTransaction 事务操作
func (r *MongoRepository[T]) WithTransaction(ctx context.Context, fn func(session mongo.SessionContext) error, opts *options.TransactionOptions) error {
	session, err := r.client.StartSession()
	if err != nil {
		return errors.Wrap(err, "start session failed")
	}
	defer session.EndSession(ctx)

	if opts == nil {
		opts = options.Transaction().
			SetReadConcern(readconcern.Snapshot()).
			SetWriteConcern(writeconcern.Majority())
	}
	_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
		return nil, fn(sc)
	}, opts)

	return err
}

// Transaction 事务操作（带返回值）
func (r *MongoRepository[T]) Transaction(ctx context.Context, fn func(session mongo.SessionContext) (any, error), opts *options.TransactionOptions) (any, error) {
	session, err := r.client.StartSession()
	if err != nil {
		return nil, errors.Wrap(err, "start session failed")
	}
	defer session.EndSession(ctx)

	if opts == nil {
		opts = options.Transaction().
			SetReadConcern(readconcern.Snapshot()).
			SetWriteConcern(writeconcern.Majority())
	}

	return session.WithTransaction(ctx, fn, opts)
}

// ==================== 变更流操作 ====================

// Watch 监听变更流
func (r *MongoRepository[T]) Watch(ctx context.Context, pipeline any, opts ...*options.ChangeStreamOptions) (*mongo.ChangeStream, error) {
	cs, err := r.collection.Watch(ctx, pipeline, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "watch changes failed")
	}
	return cs, nil
}

// WatchChanges 监听变更（简化版）
func (r *MongoRepository[T]) WatchChanges(ctx context.Context, pipeline any) (*mongo.ChangeStream, error) {
	opts := options.ChangeStream().
		SetFullDocument(options.UpdateLookup).
		SetMaxAwaitTime(2 * time.Second)
	return r.Watch(ctx, pipeline, opts)
}

// ==================== 实用方法 ====================

// GetCollection 获取原始集合
func (r *MongoRepository[T]) GetCollection() *mongo.Collection {
	return r.collection
}

// GetDatabase 获取数据库
func (r *MongoRepository[T]) GetDatabase() *mongo.Database {
	return r.client.Database(r.dbName)
}

// CreateCollection 创建集合
func (r *MongoRepository[T]) CreateCollection(ctx context.Context, opts ...*options.CreateCollectionOptions) error {
	err := r.client.Database(r.dbName).CreateCollection(ctx, r.collName, opts...)
	if err != nil {
		return errors.Wrap(err, "create collection failed")
	}
	return nil
}

// DropCollection 删除集合
func (r *MongoRepository[T]) DropCollection(ctx context.Context) error {
	err := r.collection.Drop(ctx)
	if err != nil {
		return errors.Wrap(err, "drop collection failed")
	}
	return nil
}

// Exists 检查集合是否存在
func (r *MongoRepository[T]) Exists(ctx context.Context) (bool, error) {
	names, err := r.client.Database(r.dbName).ListCollectionNames(ctx, bson.M{"name": r.collName})
	if err != nil {
		return false, errors.Wrap(err, "check collection exists failed")
	}
	return len(names) > 0, nil
}

// ==================== 分页查询 ====================

// Paginate 分页查询
func (r *MongoRepository[T]) Paginate(ctx context.Context, filter any, page, pageSize int64, sort any, opts ...*options.FindOptions) ([]T, int64, error) {
	// 计算总数
	total, err := r.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	findOpts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize)

	if sort != nil {
		findOpts.SetSort(sort)
	}

	// 合并额外选项
	for _, opt := range opts {
		opt.SetSkip((page - 1) * pageSize)
		opt.SetLimit(pageSize)
		if sort != nil && opt.Sort == nil {
			opt.SetSort(sort)
		}
	}

	items, err := r.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// ==================== 辅助函数 ====================

// ToObjectID 转换字符串为ObjectID
func ToObjectID(idStr string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(idStr)
}

// MustObjectID 转换字符串为ObjectID（失败则panic）
func MustObjectID(idStr string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		panic(fmt.Sprintf("invalid object id: %s", idStr))
	}
	return id
}

// IsDuplicateKeyError 检查是否是重复键错误
func IsDuplicateKeyError(err error) bool {
	if writeErr, ok := err.(mongo.WriteException); ok {
		for _, we := range writeErr.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return false
}

// IsNotFoundError 检查是否是文档不存在错误
func IsNotFoundError(err error) bool {
	return err == mongo.ErrNoDocuments
}
