package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func relationFixture(t *testing.T) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s = inWorkspace(t, s, "default")
	alTeam(t, s)
	for range 4 {
		if _, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Dependency"}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestRelationDAG(t *testing.T) {
	for _, kind := range []string{"blocks", "blocked_by"} {
		t.Run(kind, func(t *testing.T) {
			s := relationFixture(t)
			ctx := context.Background()
			for _, edge := range [][2]string{{"AL-1", "AL-2"}, {"AL-2", "AL-3"}} {
				if err := s.Relation(ctx, edge[0], "blocks", edge[1], false); err != nil {
					t.Fatal(err)
				}
			}
			from, to := "AL-3", "AL-1"
			if kind == "blocked_by" {
				from, to = to, from
			}
			err := s.Relation(ctx, from, kind, to, false)
			if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
				t.Fatalf("accepted dependency cycle: %v", err)
			}
		})
	}
}

func TestRelationDAGSQL(t *testing.T) {
	for _, query := range []string{
		`INSERT INTO relations VALUES (3,'blocks',1)`,
		`INSERT INTO relations VALUES (2,'blocks',1)`,
		`UPDATE relations SET issue_id=3 WHERE issue_id=1 AND other_id=2`,
		`INSERT INTO relations VALUES (3,'blocks',4),(4,'blocks',1)`,
		`UPDATE relations SET other_id=CASE issue_id WHEN 1 THEN 4 ELSE 2 END`,
	} {
		t.Run(query, func(t *testing.T) {
			s := relationFixture(t)
			if _, err := s.db.Exec(`INSERT INTO relations VALUES (1,'blocks',2),(2,'blocks',3)`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(query); err == nil {
				t.Fatal("SQL accepted dependency cycle")
			}
			var edges string
			if err := s.db.QueryRow(`SELECT group_concat(issue_id || ':' || other_id, ',') FROM (SELECT * FROM relations ORDER BY issue_id,other_id)`).Scan(&edges); err != nil || edges != "1:2,2:3" {
				t.Fatal("failed write was not atomic", edges, err)
			}
		})
	}
}

// Removing the triggers proves the Service rejects cycles independently.
func TestRelationDAGServiceWithoutTriggers(t *testing.T) {
	s := relationFixture(t)
	if _, err := s.db.Exec(`DROP TRIGGER relations_blocks_insert; DROP TRIGGER relations_blocks_update`); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, edge := range [][2]string{{"AL-1", "AL-2"}, {"AL-1", "AL-3"}, {"AL-2", "AL-4"}, {"AL-3", "AL-4"}} {
		for range 2 {
			if err := s.Relation(ctx, edge[0], "blocks", edge[1], false); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, edge := range [][2]string{{"AL-2", "AL-1"}, {"AL-4", "AL-1"}, {"AL-1", "AL-1"}} {
		if err := s.Relation(ctx, edge[0], "blocks", edge[1], false); !errors.Is(err, ErrRelationCycle) {
			t.Fatal("expected cycle error", err)
		}
	}
	if err := s.Relation(ctx, "AL-4", "related", "AL-1", false); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Relation(ctx, "AL-4", "blocked_by", "AL-3", true); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Relation(ctx, "AL-4", "blocks", "AL-3", false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM relations`).Scan(&count); err != nil || count != 5 {
		t.Fatal(count, err)
	}
}

func TestRelationDAGSQLUpdates(t *testing.T) {
	s := relationFixture(t)
	for _, query := range []string{
		`INSERT INTO relations VALUES (1,'blocks',2),(1,'blocks',3),(2,'blocks',4),(3,'blocks',4),(1,'related',4)`,
		`INSERT INTO relations VALUES (1,'blocks',2) ON CONFLICT DO NOTHING`,
		`UPDATE relations SET kind=kind`,
		`DELETE FROM relations WHERE issue_id=3 AND other_id=4`,
		`UPDATE relations SET issue_id=4,other_id=2 WHERE issue_id=2 AND other_id=4`,
		`INSERT INTO relations VALUES (1,'blocks',4)`,
	} {
		if _, err := s.db.Exec(query); err != nil {
			t.Fatal(query, err)
		}
	}
	// A related edge is not part of reachability until converted to blocks.
	if _, err := s.db.Exec(`INSERT INTO relations VALUES (2,'related',4)`); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`UPDATE relations SET kind='blocks' WHERE issue_id=2 AND kind='related' AND other_id=4`,
		`UPDATE relations SET other_id=1 WHERE issue_id=4 AND kind='blocks'`,
		`INSERT OR REPLACE INTO relations VALUES (2,'blocks',4)`,
		`INSERT OR IGNORE INTO relations VALUES (2,'blocks',4)`,
		`INSERT INTO relations VALUES (1,'blocks',1)`,
	} {
		if _, err := s.db.Exec(query); err == nil {
			t.Fatal("accepted cycle", query)
		}
	}
}

func TestRelationDAGPrivatePath(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	for _, team := range []CreateTeam{{Key: "PUB", Name: "Public"}, {Key: "SEC", Name: "Secret", Private: true}} {
		if _, err := s.CreateTeam(admin, team); err != nil {
			t.Fatal(err)
		}
	}
	for _, team := range []string{"PUB", "SEC", "PUB"} {
		if _, err := s.Create(admin, CreateIssue{Team: team, Title: "Dependency"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, edge := range [][2]string{{"PUB-1", "SEC-1"}, {"SEC-1", "PUB-2"}} {
		if err := s.Relation(admin, edge[0], "blocks", edge[1], false); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Relation(member, "PUB-2", "blocks", "PUB-1", false); !errors.Is(err, ErrRelationCycle) {
		t.Fatal(err)
	}
	if err := s.Relation(member, "PUB-2", "blocks", "SEC-1", false); !errors.Is(err, ErrNotFound) {
		t.Fatal("access must precede cycle check", err)
	}
}

func TestRelationDAGUpgrade(t *testing.T) {
	for _, cyclic := range []bool{false, true} {
		t.Run(fmt.Sprint(cyclic), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "upgrade.db")
			root, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			s := inWorkspace(t, root, "default")
			alTeam(t, s)
			for range 3 {
				if _, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Preserve", Description: "Body"}); err != nil {
					t.Fatal(err)
				}
			}
			// Version 13 is the shipped schema immediately before dependency constraints.
			if _, err := root.db.Exec(`DROP TRIGGER relations_blocks_insert; DROP TRIGGER relations_blocks_update; PRAGMA user_version=13;
    INSERT INTO relations VALUES (1,'blocks',2),(2,'blocks',3),(1,'related',3)`); err != nil {
				t.Fatal(err)
			}
			if cyclic {
				if _, err := root.db.Exec(`INSERT INTO relations VALUES (3,'blocks',1)`); err != nil {
					t.Fatal(err)
				}
			}
			before, err := s.Details(context.Background(), "AL-1")
			if err != nil {
				t.Fatal(err)
			}
			root.Close()
			upgraded, err := Open(path)
			if cyclic {
				if upgraded != nil {
					upgraded.Close()
				}
				if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
					t.Fatal("legacy cycle must fail upgrade", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer upgraded.Close()
				s, err = upgraded.InWorkspace(context.Background(), "default")
				if err != nil {
					t.Fatal(err)
				}
				after, err := s.Details(context.Background(), "AL-1")
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("upgrade changed issue", after, err)
				}
				fresh := relationFixture(t)
				if !reflect.DeepEqual(structure(t, upgraded.db), structure(t, fresh.db)) {
					t.Fatal("fresh/upgrade schema mismatch")
				}
				if _, err := upgraded.db.Exec(`INSERT INTO relations VALUES (3,'blocks',1)`); err == nil {
					t.Fatal("upgraded insert accepted cycle")
				}
				if _, err := upgraded.db.Exec(`UPDATE relations SET other_id=1 WHERE issue_id=2 AND kind='blocks'`); err == nil {
					t.Fatal("upgraded update accepted cycle")
				}
			}
			for _, file := range []string{path, filepath.Join(dir, "backups", "before-migration-14.db")} {
				db, err := sql.Open("sqlite", "file:"+file+"?mode=ro")
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				wantVersion := 13
				wantCount := 3
				if cyclic {
					wantCount = 4
				} else if file == path {
					wantVersion = len(migrations)
				}
				if got := userVersion(t, db); got != wantVersion {
					t.Fatal("version", got, wantVersion)
				}
				var count int
				if err := db.QueryRow(`SELECT count(*) FROM relations`).Scan(&count); err != nil || count != wantCount {
					t.Fatal("relations changed", count, err)
				}
				if cyclic {
					if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name LIKE 'relations_blocks_%'`).Scan(&count); err != nil || count != 0 {
						t.Fatal("failed migration left triggers", count, err)
					}
				}
			}
		})
	}
}

func TestRelationDAGConcurrent(t *testing.T) {
	s := relationFixture(t)
	var seq int
	var name, path string
	if err := s.db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	other, err = other.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i, writer := range []*Service{s, other} {
		wg.Go(func() {
			<-start
			from, to := "AL-1", "AL-2"
			if i == 1 {
				from, to = to, from
			}
			results <- writer.Relation(context.Background(), from, "blocks", to, false)
		})
	}
	close(start)
	wg.Wait()
	close(results)
	successes, rejected := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrRelationCycle):
			rejected++
		default:
			t.Fatal(err)
		}
	}
	if successes != 1 || rejected != 1 {
		t.Fatal("opposing concurrent writes", successes, rejected)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM relations`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
