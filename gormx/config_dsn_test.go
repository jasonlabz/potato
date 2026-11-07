package gormx

import (
	"strings"
	"testing"
)

// TestGenDSNArgsAndSchema 覆盖各类型的 DSN 形状、参数拼接与转义规则。
// 断言的是「拼出来的字符串」，驱动侧的解析行为由各自的 Parse 函数保证。
func TestGenDSNArgsAndSchema(t *testing.T) {
	cases := []struct {
		name string
		conn *Connection
		want string
	}{
		{
			name: "DM 原样拼接不转义",
			conn: &Connection{DBType: DatabaseTypeDM, Host: "h", Port: 5236, Username: "u", Password: "p", Database: "s",
				Args: []ARG{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}},
			want: "dm://u:p@h:5236?schema=s&a=1&b=2",
		},
		{
			name: "PostgreSQL schema 加引号 + 参数空格加引号",
			conn: &Connection{DBType: DatabaseTypePostgres, Host: "h", Port: 5432, Username: "u", Password: "p", Database: "d",
				Schema: "app", Args: []ARG{{Name: "application_name", Value: "my app"}}},
			want: "user=u password=p host=h port=5432 dbname=d search_path='app' application_name='my app'",
		},
		{
			name: "PostgreSQL 密码含单引号转义",
			conn: &Connection{DBType: DatabaseTypePostgres, Host: "h", Port: 5432, Username: "u", Password: `p'ss`, Database: "d"},
			want: `user=u password='p\'ss' host=h port=5432 dbname=d`,
		},
		{
			name: "Oracle 密码特殊字符转义",
			conn: &Connection{DBType: DatabaseTypeOracle, Host: "h", Port: 1521, Username: "u", Password: "p@ss/w", Database: "svc"},
			want: "oracle://u:p@ss%2Fw@h:1521/svc",
		},
		{
			name: "Oracle 参数名带空格、值转义",
			conn: &Connection{DBType: DatabaseTypeOracle, Host: "h", Port: 1521, Username: "u", Password: "p", Database: "svc",
				Args: []ARG{{Name: "CONNECTION TIMEOUT", Value: "90"}, {Name: "TRACE FILE", Value: "/tmp/a b.log"}}},
			want: "oracle://u:p@h:1521/svc?CONNECTION TIMEOUT=90&TRACE FILE=%2Ftmp%2Fa+b.log",
		},
		{
			name: "Oracle SERVER 逗号拆多值且不转义",
			conn: &Connection{DBType: DatabaseTypeOracle, Host: "h", Port: 1521, Username: "u", Password: "p", Database: "svc",
				Args: []ARG{{Name: "SERVER", Value: "s1:1521, s2:1521"}}},
			want: "oracle://u:p@h:1521/svc?SERVER=s1:1521&SERVER=s2:1521",
		},
		{
			name: "MySQL 库名与参数按 URL 编码",
			conn: &Connection{DBType: DatabaseTypeMySQL, Host: "h", Port: 3306, Username: "u", Password: "p", Database: "db/name",
				Args: []ARG{{Name: "charset", Value: "utf8"}, {Name: "loc", Value: "Asia/Shanghai"}}},
			want: "u:p@tcp(h:3306)/db%2Fname?charset=utf8&loc=Asia%2FShanghai",
		},
		{
			name: "SQL Server 必须带 odbc 前缀才能保住含分号的值",
			conn: &Connection{DBType: DatabaseTypeSqlserver, Host: "h", Port: 1433, Username: "u", Password: "p;w", Database: "d",
				Args: []ARG{{Name: "app name", Value: "a;b"}}},
			want: "odbc:user id=u;password={p;w};server=h;port=1433;database=d;app name={a;b}",
		},
		{
			name: "SQL Server 花括号内的右括号双写",
			conn: &Connection{DBType: DatabaseTypeSqlserver, Host: "h", Port: 1433, Username: "u", Password: "p}w", Database: "d"},
			want: "odbc:user id=u;password={p}}w};server=h;port=1433;database=d",
		},
		{
			name: "IPv6 地址补方括号",
			conn: &Connection{DBType: DatabaseTypeOracle, Host: "::1", Port: 1521, Username: "u", Password: "p", Database: "svc"},
			want: "oracle://u:p@[::1]:1521/svc",
		},
		{
			name: "DM 未配置 database 时省略 schema 段",
			conn: &Connection{DBType: DatabaseTypeDM, Host: "h", Port: 5236, Username: "u", Password: "p",
				Args: []ARG{{Name: "a", Value: "1"}}},
			want: "dm://u:p@h:5236?a=1",
		},
		{
			name: "PostgreSQL 未配置 schema 时不出空 search_path",
			conn: &Connection{DBType: DatabaseTypePostgres, Host: "h", Port: 5432, Username: "u", Password: "p", Database: "d",
				Args: []ARG{{Name: "sslmode", Value: "disable"}}},
			want: "user=u password=p host=h port=5432 dbname=d sslmode=disable",
		},
		{
			name: "SQLite 默认内存库追加参数用 &",
			conn: &Connection{DBType: DatabaseTypeSQLite, Args: []ARG{{Name: "mode", Value: "memory"}}},
			want: "file::memory:?cache=shared&mode=memory",
		},
		{
			name: "SQLite 配置 database 时按文件路径打开",
			conn: &Connection{DBType: DatabaseTypeSQLite, Database: "app.db"},
			want: "app.db",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.conn.genDSN(); got != tc.want {
				t.Fatalf("genDSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGenDSNPasswordAt 密码含 @ 不应破坏 URL 结构（各驱动都以最后一个 @ 分界）
func TestGenDSNPasswordAt(t *testing.T) {
	ora := &Connection{DBType: DatabaseTypeOracle, Host: "h", Port: 1521, Username: "u", Password: "p#ss?w@rd/1", Database: "svc"}
	if got, want := ora.genDSN(), "oracle://u:p%23ss%3Fw@rd%2F1@h:1521/svc"; got != want {
		t.Fatalf("Oracle genDSN() = %q, want %q", got, want)
	}
	mysql := &Connection{DBType: DatabaseTypeMySQL, Host: "h", Port: 3306, Username: "u", Password: "p@ss/w", Database: "d"}
	if got, want := mysql.genDSN(), "u:p@ss/w@tcp(h:3306)/d"; got != want {
		t.Fatalf("MySQL genDSN() = %q, want %q", got, want)
	}
}

// TestGenDSNMissingHost 缺少 host/port 时不应产生无效 DSN
func TestGenDSNMissingHost(t *testing.T) {
	for _, bt := range []DatabaseType{DatabaseTypeOracle, DatabaseTypeMySQL, DatabaseTypePostgres, DatabaseTypeSqlserver, DatabaseTypeDM} {
		if got := (&Connection{DBType: bt, Username: "u"}).genDSN(); got != "" {
			t.Errorf("%s genDSN() = %q, want empty", bt, got)
		}
	}
}

// TestDSNDialectsCoverage 每种 DatabaseType 都必须登记 DSN 描述，且模板里的占位符能被完全替换
func TestDSNDialectsCoverage(t *testing.T) {
	all := []DatabaseType{
		DatabaseTypeMySQL, DatabaseTypePostgres, DatabaseTypeSqlserver,
		DatabaseTypeOracle, DatabaseTypeSQLite, DatabaseTypeDM,
	}
	conn := &Connection{Username: "u", Password: "p", Host: "h", Port: 1, Database: "d", Schema: "s",
		Args: []ARG{{Name: "k", Value: "v"}}}
	for _, dbType := range all {
		dialect, ok := dsnDialects[dbType]
		if !ok {
			t.Errorf("%s 未登记 dsnDialects", dbType)
			continue
		}
		conn.DBType = dbType
		got := dialect.appendArgs(dialect.render(conn), conn.Args)
		if strings.ContainsAny(got, "[]{}") {
			t.Errorf("%s 渲染后残留占位符或方括号: %q", dbType, got)
		}
	}
}

// TestGenDSNExplicitDSN 显式配置 dsn 时原样返回
func TestGenDSNExplicitDSN(t *testing.T) {
	c := &Connection{DBType: DatabaseTypeOracle, DSN: "oracle://custom/dsn", Host: "ignored", Port: 1}
	if got := c.genDSN(); got != "oracle://custom/dsn" {
		t.Fatalf("genDSN() = %q, want 显式 DSN 原样返回", got)
	}
}
