package gormx

import (
	"net/url"
	"strconv"
	"strings"
)

// 本文件负责把 Connection 拼装成各驱动要求的 DSN。
//
// 一个数据库类型的 DSN 由三部分描述，全部集中在 dsnDialects 里，genDSN 不含任何 per-type 分支：
//   - 形状 template：用 {user} {password} {host} {port} {database} {schema} 命名占位符描述
//     DSN 长什么样；[ ] 包起来的片段在该片段内占位符为空时整段省略（如没配 schema 就不输出 ?schema=）；
//   - 编码 enc：同一个位置在不同驱动里的转义规则不同，按位置登记，与形状分开维护；
//   - 参数 argSep / argPrefix：?, &, 空格, ; 这类参数拼接规则。
//
// 之所以用命名占位符而不是 %s：%s 的位置与个数必须和 Sprintf 的实参严格对齐，改模板时一旦漏同步，
// 多出来的实参不会报错，而是变成 %!(EXTRA string=...) 混进 DSN。命名占位符没有这个耦合，
// 模板里没有的占位符自然不会被用到。

const (
	phUser     = "{user}"
	phPassword = "{password}"
	phHost     = "{host}"
	phPort     = "{port}"
	phDatabase = "{database}"
	phSchema   = "{schema}"

	// defaultSQLiteDSN 未配置 database 时的共享内存库
	defaultSQLiteDSN = "file::memory:?cache=shared"
)

// dsnEncoder 描述 DSN 中各位置的值该怎么转义；字段为 nil 表示原样直传。
type dsnEncoder struct {
	user     func(string) string
	password func(string) string
	host     func(string) string
	database func(string) string
	schema   func(string) string
	// arg 把配置项编码成若干 "name=value" 片段，返回多个表示一个配置项可展开成多个键值对
	arg func(name, value string) []string
}

// dsnDialect 一种数据库的 DSN 形状、参数拼接方式与取值编码。
type dsnDialect struct {
	// template 命名占位符模板，[ ] 段在其中的占位符为空时整段省略
	template string
	// defaultDatabase Database 为空时的兜底值（仅 SQLite 使用）
	defaultDatabase string
	// argSep 参数之间的分隔符；argPrefix 首个参数的连接符
	argSep, argPrefix string
	// argPrefixInQuery 模板渲染结果里已出现 ? 时改用的连接符，URL 风格类型填 &
	argPrefixInQuery string
	enc              dsnEncoder
}

// dsnDialects 各数据库类型的 DSN 描述；未登记的类型视为不支持。
var dsnDialects = map[DatabaseType]dsnDialect{
	// SQLite：DSN 就是文件路径，没有 host/port，未配置 database 时落到共享内存库。
	// 参数用 ?x=1&y=2 追加；内存库的 DSN 自带 ?cache=shared，所以首个参数要用 &。
	DatabaseTypeSQLite: {
		template:         phDatabase,
		defaultDatabase:  defaultSQLiteDSN,
		argSep:           "&",
		argPrefix:        "?",
		argPrefixInQuery: "&",
		enc: dsnEncoder{
			database: ident,
			arg:      argPlain,
		},
	},
	// MySQL：go-sql-driver 的 dbname 用 url.PathEscape 编解码，参数值用 QueryUnescape 反解；
	// 用户名和密码在 DSN 里没有转义机制，含 @ 属驱动自身限制。
	DatabaseTypeMySQL: {
		template:         "{user}:{password}@tcp({host}:{port})/{database}",
		argSep:           "&",
		argPrefix:        "?",
		argPrefixInQuery: "&",
		enc: dsnEncoder{
			host:     hostAuthority,
			database: urlSegment,
			arg:      argQuery,
		},
	},
	// PostgreSQL：key=value 形式，值含空白/引号/反斜杠时用单引号包裹并转义（pgx 与 lib/pq 都支持）。
	DatabaseTypePostgres: {
		// search_path 始终加引号，支持 "app, public" 这类带空格的写法；schema 为空时整段省略
		template:  "user={user} password={password} host={host} port={port} dbname={database}[ search_path='{schema}']",
		argSep:    " ",
		argPrefix: " ",
		enc: dsnEncoder{
			user:     quotePG,
			password: quotePG,
			database: quotePG,
			schema:   pgEscape,
			arg:      argPG,
		},
	},
	// SQL Server：; 分隔，值含 ; { } 时用 {} 包裹，内部的 } 双写转义，否则连接串会被截断。
	// DSN 必须带 odbc: 前缀：不带前缀时 msdsn 按 ADO 规则无脑以 ; 切分，完全不认花括号，
	// 值里含 ; 会被截断（密码 p;w 变成 {p）；带前缀才走支持 {} 与 }} 的状态机。
	DatabaseTypeSqlserver: {
		template:  "odbc:user id={user};password={password};server={host};port={port};database={database}",
		argSep:    ";",
		argPrefix: ";",
		enc: dsnEncoder{
			user:     quoteMSSQL,
			password: quoteMSSQL,
			database: quoteMSSQL,
			arg:      argMSSQL,
		},
	},
	// 达梦：dm:// URL 形式，驱动的 parseDSN 只按 @ 和 : 做字符串切分、不做 URL 反解，
	// 因此这里必须原样传递，转义反而会导致认证失败。达梦的库即 schema，落在 ?schema= 上。
	DatabaseTypeDM: {
		template:         "dm://{user}:{password}@{host}:{port}[?schema={database}]",
		argSep:           "&",
		argPrefix:        "?",
		argPrefixInQuery: "&",
		enc: dsnEncoder{
			arg: argPlain,
		},
	},
	// Oracle：go-ora 用 url.Parse 解析 DSN，user/password/service 必须转义；
	// 参数值需 QueryEscape，SERVER 例外（可能形如 (DESCRIPTION=...)，且逗号表示多节点）。
	DatabaseTypeOracle: {
		template:         "oracle://{user}:{password}@{host}:{port}/{database}",
		argSep:           "&",
		argPrefix:        "?",
		argPrefixInQuery: "&",
		enc: dsnEncoder{
			user:     urlSegment,
			password: urlSegment,
			database: urlSegment,
			host:     hostAuthority,
			arg:      argOracle,
		},
	},
}

// ident 原样返回，用于驱动不做任何反解的位置。
func ident(s string) string { return s }

// urlSegment 转义 URL 中的用户信息与路径片段。
func urlSegment(s string) string { return url.PathEscape(s) }

// hostAuthority 给 IPv6 地址补方括号，用于模板里 host:port 合并成 authority 的场景。
func hostAuthority(h string) string {
	if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
		return "[" + h + "]"
	}
	return h
}

// pgEscape 转义 PostgreSQL key=value 形式中的引号与反斜杠。
func pgEscape(v string) string {
	return strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v)
}

// quotePG 在值含空白、单引号或反斜杠时用单引号包裹并转义。
func quotePG(v string) string {
	if v == "" || !strings.ContainsAny(v, " '\\") {
		return v
	}
	return "'" + pgEscape(v) + "'"
}

// quoteMSSQL 在值含 ; { } 或首尾空白时用花括号包裹，内部的 } 双写。
func quoteMSSQL(v string) string {
	if v == "" || (v == strings.TrimSpace(v) && !strings.ContainsAny(v, ";{}")) {
		return v
	}
	return "{" + strings.ReplaceAll(v, "}", "}}") + "}"
}

// argPlain 原样拼接，用于不做反解的驱动（SQLite、达梦）。
func argPlain(name, value string) []string {
	return []string{name + "=" + value}
}

// argQuery 按 URL query 编码，用于会 QueryUnescape 的驱动（MySQL）。
func argQuery(name, value string) []string {
	return []string{name + "=" + url.QueryEscape(value)}
}

// argPG 同 PostgreSQL 的 key=value 规则。
func argPG(name, value string) []string {
	return []string{name + "=" + quotePG(value)}
}

// argMSSQL 同 SQL Server 连接串规则。
func argMSSQL(name, value string) []string {
	return []string{name + "=" + quoteMSSQL(value)}
}

// argOracle 对齐 go-ora 自身 BuildUrl 的行为：值按逗号拆成多个同名参数，
// 逐段 QueryEscape；SERVER 保持原样，否则 (DESCRIPTION=...) 会被转义成解析不了的形式。
func argOracle(name, value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.EqualFold(name, "SERVER") {
			out = append(out, name+"="+part)
			continue
		}
		out = append(out, name+"="+url.QueryEscape(part))
	}
	return out
}

// genDSN 按「模板 + 编码表」拼装 DSN；显式配置了 DSN 时直接返回。
func (c *Connection) genDSN() (dsn string) {
	if c.DSN != "" {
		return c.DSN
	}
	dialect, ok := dsnDialects[c.DBType]
	if !ok {
		return ""
	}
	// 模板里用到 host 的类型必须配全 host/port，否则拼出来的 DSN 连不上
	if strings.Contains(dialect.template, phHost) && (c.Host == "" || c.Port == 0) {
		return ""
	}

	dsn = dialect.appendArgs(dialect.render(c), c.Args)
	c.DSN = dsn
	return dsn
}

// render 用 Connection 中的值填充模板。
func (d dsnDialect) render(c *Connection) string {
	database := c.Database
	if database == "" {
		database = d.defaultDatabase
	}
	values := map[string]string{
		phUser:     apply(d.enc.user, c.Username),
		phPassword: apply(d.enc.password, c.Password),
		phHost:     apply(d.enc.host, c.Host),
		phPort:     strconv.Itoa(c.Port),
		phDatabase: apply(d.enc.database, database),
		phSchema:   apply(d.enc.schema, c.Schema),
	}
	return substitute(expandOptional(d.template, values), values)
}

// appendArgs 追加 Args；渲染结果里已经有 ? 时改用 argPrefixInQuery（URL 风格类型的 &）。
func (d dsnDialect) appendArgs(dsn string, args []ARG) string {
	if len(args) == 0 {
		return dsn
	}
	pairs := make([]string, 0, len(args))
	for _, arg := range args {
		encode := d.enc.arg
		if encode == nil {
			encode = argPlain
		}
		pairs = append(pairs, encode(arg.Name, arg.Value)...)
	}
	prefix := d.argPrefix
	if d.argPrefixInQuery != "" && strings.Contains(dsn, "?") {
		prefix = d.argPrefixInQuery
	}
	return dsn + prefix + strings.Join(pairs, d.argSep)
}

// expandOptional 展开 [ ... ] 可选片段：片段内的占位符都取到非空值则去掉方括号保留内容，否则整段删除。
func expandOptional(template string, values map[string]string) string {
	if !strings.Contains(template, "[") {
		return template
	}
	var b strings.Builder
	for {
		start := strings.IndexByte(template, '[')
		if start < 0 {
			b.WriteString(template)
			return b.String()
		}
		end := strings.IndexByte(template[start:], ']')
		if end < 0 {
			b.WriteString(template)
			return b.String()
		}
		b.WriteString(template[:start])
		if segmentReady(template[start+1:start+end], values) {
			b.WriteString(template[start+1 : start+end])
		}
		template = template[start+end+1:]
	}
}

// segmentReady 判断片段内的占位符是否都取到了非空值。
func segmentReady(segment string, values map[string]string) bool {
	for placeholder, value := range values {
		if value == "" && strings.Contains(segment, placeholder) {
			return false
		}
	}
	return true
}

// substitute 一次性替换全部占位符，避免替换进来的值被二次当成占位符。
func substitute(template string, values map[string]string) string {
	pairs := make([]string, 0, len(values)*2)
	for placeholder, value := range values {
		pairs = append(pairs, placeholder, value)
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

// apply 应用编码函数，未登记（nil）的位置原样直传。
func apply(encode func(string) string, v string) string {
	if encode == nil {
		return v
	}
	return encode(v)
}
