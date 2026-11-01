package gormx

import "testing"

func TestGenDSNArgsAndSchema(t *testing.T) {
	dm := &Connection{DBType: DatabaseTypeDM, Host: "h", Port: 5236, Username: "u", Password: "p", Database: "s",
		Args: []ARG{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}}
	if got, want := dm.genDSN(), "dm://u:p@h:5236?schema=s&a=1&b=2"; got != want {
		t.Fatalf("DM genDSN() = %q, want %q", got, want)
	}
	pg := &Connection{DBType: DatabaseTypePostgres, Host: "h", Port: 5432, Username: "u", Password: "p", Database: "d", Schema: "app"}
	if got, want := pg.genDSN(), "user=u password=p host=h port=5432 dbname=d search_path='app'"; got != want {
		t.Fatalf("PG genDSN() = %q, want %q", got, want)
	}
}
