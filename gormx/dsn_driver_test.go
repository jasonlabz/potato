package gormx

import (
	"net/url"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/microsoft/go-mssqldb/msdsn"
)

// TestDriverParseGeneratedDSN 用各驱动自带的解析函数回读 genDSN 的结果：
// 只断言字符串容易「自说自话」，这里锁住的是「驱动真的能从我们拼的 DSN 里取回原值」，
// 驱动升级（如 go-mssqldb 1.7 → 1.9）改动 DSN 规则时会在这里暴露。
func TestDriverParseGeneratedDSN(t *testing.T) {
	t.Run("mysql", func(t *testing.T) {
		conn := &Connection{DBType: DatabaseTypeMySQL, Host: "h", Port: 3306, Username: "u", Password: "p@ss/w",
			Database: "db/name", Args: []ARG{{Name: "charset", Value: "utf8"}, {Name: "loc", Value: "Asia/Shanghai"}}}
		cfg, err := mysqldriver.ParseDSN(conn.genDSN())
		if err != nil {
			t.Fatalf("ParseDSN: %v", err)
		}
		if cfg.User != "u" || cfg.Passwd != "p@ss/w" || cfg.DBName != "db/name" ||
			cfg.Params["charset"] != "utf8" || cfg.Loc.String() != "Asia/Shanghai" || cfg.Addr != "h:3306" {
			t.Fatalf("解析结果不符: %+v", cfg)
		}
	})

	t.Run("postgres", func(t *testing.T) {
		conn := &Connection{DBType: DatabaseTypePostgres, Host: "h", Port: 5432, Username: "u", Password: `p'ss w`,
			Database: "d", Schema: "app", Args: []ARG{{Name: "application_name", Value: "my app"}}}
		cfg, err := pgx.ParseConfig(conn.genDSN())
		if err != nil {
			t.Fatalf("ParseConfig: %v", err)
		}
		if cfg.User != "u" || cfg.Password != `p'ss w` || cfg.Database != "d" ||
			cfg.RuntimeParams["application_name"] != "my app" || cfg.RuntimeParams["search_path"] != "app" {
			t.Fatalf("解析结果不符: %+v", cfg)
		}
	})

	t.Run("sqlserver", func(t *testing.T) {
		conn := &Connection{DBType: DatabaseTypeSqlserver, Host: "h", Port: 1433, Username: "u", Password: "p;w",
			Database: "d", Args: []ARG{{Name: "app name", Value: "a;b"}}}
		cfg, err := msdsn.Parse(conn.genDSN())
		if err != nil {
			t.Fatalf("msdsn.Parse: %v", err)
		}
		if cfg.User != "u" || cfg.Password != "p;w" || cfg.Database != "d" ||
			cfg.Parameters["app name"] != "a;b" || cfg.Host != "h" || cfg.Port != 1433 {
			t.Fatalf("解析结果不符: %+v", cfg)
		}
	})

	t.Run("oracle", func(t *testing.T) {
		conn := &Connection{DBType: DatabaseTypeOracle, Host: "h", Port: 1521, Username: "u", Password: "p@ss/w",
			Database: "svc", Args: []ARG{{Name: "CONNECTION TIMEOUT", Value: "90"}}}
		u, err := url.Parse(conn.genDSN())
		if err != nil {
			t.Fatalf("url.Parse: %v", err)
		}
		pwd, _ := u.User.Password()
		if u.User.Username() != "u" || pwd != "p@ss/w" || u.Path != "/svc" ||
			u.Query().Get("CONNECTION TIMEOUT") != "90" {
			t.Fatalf("解析结果不符: %s", u.String())
		}
	})
}
