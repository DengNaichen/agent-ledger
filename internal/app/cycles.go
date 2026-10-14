package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	_ "time/tzdata"
)

type CycleSettings struct {
	Enabled       bool   `json:"enabled"`
	Timezone      string `json:"timezone"`
	StartDate     string `json:"start_date"`
	DurationWeeks int    `json:"duration_weeks"`
	UpcomingCount int    `json:"upcoming_count"`
}
type ConfigureCycles struct {
	Team string `json:"team"`
	CycleSettings
}
type Cycle struct {
	Timezone    string  `json:"timezone"`
	ID          int64   `json:"id"`
	Team        string  `json:"team"`
	Number      int     `json:"number"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	StartsAt    string  `json:"starts_at"`
	EndsAt      string  `json:"ends_at"`
	CompletedAt *string `json:"completed_at"`
	State       string  `json:"state"`
	Total       int     `json:"total"`
	Done        int     `json:"done"`
	Canceled    int     `json:"canceled"`
	RolledOver  int     `json:"rolled_over"`
}
type UpdateCycle struct {
	ID          int64   `json:"id"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}
type CyclePage struct {
	Settings CycleSettings `json:"settings"`
	Cycles   []Cycle       `json:"cycles"`
}

func cycleSettings(ctx context.Context, tx *sql.Tx, team int64) (CycleSettings, error) {
	c := CycleSettings{Timezone: "UTC", DurationWeeks: 2, UpcomingCount: 2}
	err := tx.QueryRowContext(ctx, `SELECT enabled,timezone,start_date,duration_weeks,upcoming_count FROM team_cycle_settings WHERE team_id=?`, team).Scan(&c.Enabled, &c.Timezone, &c.StartDate, &c.DurationWeeks, &c.UpcomingCount)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return c, err
}

func (s *Service) ConfigureCycles(ctx context.Context, in ConfigureCycles) (CyclePage, error) {
	if err := s.checkAccess(ctx); err != nil {
		return CyclePage{}, err
	}
	team, err := s.teamID(ctx, in.Team)
	if err != nil {
		return CyclePage{}, err
	}
	if !team.Valid {
		return CyclePage{}, ErrTeamRequired
	}
	settings, err := s.teamSettings(ctx, in.Team)
	if err != nil {
		return CyclePage{}, err
	}
	if settings.MembershipEnabled && !settings.CanManage {
		return CyclePage{}, ErrTeamOwnerRequired
	}
	loc, err := time.LoadLocation(in.Timezone)
	if err != nil || in.Timezone == "" || in.Timezone == "Local" || in.DurationWeeks < 1 || in.DurationWeeks > 8 || in.UpcomingCount < 1 || in.UpcomingCount > 15 {
		return CyclePage{}, ErrCycleSettings
	}
	start, err := time.ParseInLocation(time.DateOnly, in.StartDate, loc)
	if err != nil {
		return CyclePage{}, ErrCycleSettings
	}
	now := s.now()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CyclePage{}, err
	}
	defer tx.Rollback()
	old, err := cycleSettings(ctx, tx, team.Int64)
	if err != nil {
		return CyclePage{}, err
	}
	if old.Enabled && in.Enabled && (old.Timezone != in.Timezone || old.StartDate != in.StartDate || old.DurationWeeks != in.DurationWeeks) {
		return CyclePage{}, ErrCycleSchedule
	}
	if !old.Enabled && in.Enabled {
		// A new schedule begins today or later; closed history must not overlap.
		today := now.In(loc).Format(time.DateOnly)
		if in.StartDate < today {
			return CyclePage{}, ErrCycleSettings
		}
		var overlap bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cycles WHERE team_id=? AND ends_at>?)`, team.Int64, cycleTime(start)).Scan(&overlap); err != nil {
			return CyclePage{}, err
		}
		if overlap {
			return CyclePage{}, ErrCycleSchedule
		}
	}
	if old.Enabled {
		if err = s.advanceTeamCycles(ctx, tx, team.Int64, old, now); err != nil {
			return CyclePage{}, err
		}
	}
	if old.Enabled && !in.Enabled {
		// Preserve closed snapshots and cancel only unstarted plans.
		if err = s.closeCycles(ctx, tx, team.Int64, cycleTime(now), false); err != nil {
			return CyclePage{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE issues SET cycle_id=NULL,updated_at=? WHERE cycle_id IN(SELECT id FROM cycles WHERE team_id=? AND completed_at IS NULL)`, cycleTime(now), team.Int64); err != nil {
			return CyclePage{}, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM cycles WHERE team_id=? AND completed_at IS NULL`, team.Int64); err != nil {
			return CyclePage{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO team_cycle_settings(team_id,enabled,timezone,start_date,duration_weeks,upcoming_count) VALUES(?,?,?,?,?,?) ON CONFLICT(team_id) DO UPDATE SET enabled=excluded.enabled,timezone=excluded.timezone,start_date=excluded.start_date,duration_weeks=excluded.duration_weeks,upcoming_count=excluded.upcoming_count`, team.Int64, in.Enabled, in.Timezone, in.StartDate, in.DurationWeeks, in.UpcomingCount)
	if err != nil {
		return CyclePage{}, err
	}
	if in.Enabled {
		if err = s.advanceTeamCycles(ctx, tx, team.Int64, in.CycleSettings, now); err != nil {
			return CyclePage{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return CyclePage{}, err
	}
	return s.ListCycles(ctx, in.Team)
}

func cycleTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// AdvanceCycles is a system-only maintenance operation; each team advances atomically.
func (s *Service) AdvanceCycles(ctx context.Context) error {
	if s.requireIdentity || s.workspace != 0 {
		return ErrIdentityRequired
	}
	rows, err := s.db.QueryContext(ctx, `SELECT team_id FROM team_cycle_settings WHERE enabled=1`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.advanceCycleTeam(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) advanceCycleTeam(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	cfg, err := cycleSettings(ctx, tx, id)
	if err != nil {
		return err
	}
	if cfg.Enabled {
		if err = s.advanceTeamCycles(ctx, tx, id, cfg, s.now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Service) advanceTeamCycles(ctx context.Context, tx *sql.Tx, team int64, cfg CycleSettings, now time.Time) error {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return err
	}
	start, err := time.ParseInLocation(time.DateOnly, cfg.StartDate, loc)
	if err != nil {
		return err
	}
	var lastEnd sql.NullString
	var number int
	if err = tx.QueryRowContext(ctx, `SELECT MAX(ends_at),COALESCE(MAX(number),0) FROM cycles WHERE team_id=?`, team).Scan(&lastEnd, &number); err != nil {
		return err
	}
	if lastEnd.Valid {
		last, err := time.Parse(time.RFC3339, lastEnd.String)
		if err != nil {
			return err
		}
		if last.After(start) {
			start = last.In(loc)
		}
	}
	// Fill elapsed intervals first, then keep the configured number of future cycles.
	var future int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cycles WHERE team_id=? AND completed_at IS NULL AND starts_at>?`, team, cycleTime(now)).Scan(&future); err != nil {
		return err
	}
	for !start.After(now) || future < cfg.UpcomingCount {
		end := start.AddDate(0, 0, 7*cfg.DurationWeeks)
		number++
		if _, err = tx.ExecContext(ctx, `INSERT INTO cycles(team_id,number,timezone,starts_at,ends_at) VALUES(?,?,?,?,?)`, team, number, cfg.Timezone, cycleTime(start), cycleTime(end)); err != nil {
			return err
		}
		if start.After(now) {
			future++
		}
		start = end
	}
	return s.closeCycles(ctx, tx, team, cycleTime(now), true)
}
func (s *Service) closeCycles(ctx context.Context, tx *sql.Tx, team int64, now string, roll bool) error {
	condition := "ends_at<=?"
	if !roll {
		condition = "starts_at<?"
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,ends_at FROM cycles WHERE team_id=? AND completed_at IS NULL AND `+condition+` ORDER BY starts_at`, team, now)
	if err != nil {
		return err
	}
	type closing struct {
		id  int64
		end string
	}
	var pending []closing
	for rows.Next() {
		var c closing
		if err = rows.Scan(&c.id, &c.end); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range pending {
		var total, done, canceled, moving int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(status='done'),0),COALESCE(SUM(status='canceled'),0),COALESCE(SUM(status IN('todo','agent_ready','in_progress','in_review')),0) FROM issues WHERE cycle_id=?`, c.id).Scan(&total, &done, &canceled, &moving); err != nil {
			return err
		}
		end := c.end
		if !roll {
			end = now
			moving = 0
		}
		if _, err = tx.ExecContext(ctx, `UPDATE cycles SET completed_at=?,ends_at=?,total=?,done=?,canceled=?,rolled_over=? WHERE id=?`, end, end, total, done, canceled, moving, c.id); err != nil {
			return err
		}
		if roll && moving > 0 {
			var next int64
			if err = tx.QueryRowContext(ctx, `SELECT id FROM cycles WHERE team_id=? AND starts_at>=? ORDER BY starts_at LIMIT 1`, team, c.end).Scan(&next); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE issues SET cycle_id=?,updated_at=? WHERE cycle_id=? AND status IN('todo','agent_ready','in_progress','in_review')`, next, now, c.id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ListCycles(ctx context.Context, key string) (CyclePage, error) {
	if err := s.checkAccess(ctx); err != nil {
		return CyclePage{}, err
	}
	team, err := s.teamID(ctx, key)
	if err != nil {
		return CyclePage{}, err
	}
	if !team.Valid {
		return CyclePage{}, ErrTeamRequired
	}
	if err = s.advanceCycleTeam(ctx, team.Int64); err != nil {
		return CyclePage{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CyclePage{}, err
	}
	defer tx.Rollback()
	cfg, err := cycleSettings(ctx, tx, team.Int64)
	if err != nil {
		return CyclePage{}, err
	}
	page := CyclePage{Settings: cfg, Cycles: []Cycle{}}
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.number,c.name,c.description,c.timezone,c.starts_at,c.ends_at,c.completed_at,
 CASE WHEN c.completed_at IS NULL THEN (SELECT COUNT(*) FROM issues WHERE cycle_id=c.id) ELSE c.total END,
 CASE WHEN c.completed_at IS NULL THEN (SELECT COUNT(*) FROM issues WHERE cycle_id=c.id AND status='done') ELSE c.done END,
 CASE WHEN c.completed_at IS NULL THEN (SELECT COUNT(*) FROM issues WHERE cycle_id=c.id AND status='canceled') ELSE c.canceled END,c.rolled_over
 FROM cycles c WHERE team_id=? ORDER BY starts_at DESC`, team.Int64)
	if err != nil {
		return CyclePage{}, err
	}
	now := cycleTime(s.now())
	for rows.Next() {
		c := Cycle{Team: key}
		if err = rows.Scan(&c.ID, &c.Number, &c.Name, &c.Description, &c.Timezone, &c.StartsAt, &c.EndsAt, &c.CompletedAt, &c.Total, &c.Done, &c.Canceled, &c.RolledOver); err != nil {
			rows.Close()
			return CyclePage{}, err
		}
		c.State = "upcoming"
		if c.CompletedAt != nil {
			c.State = "completed"
		} else if c.StartsAt <= now {
			c.State = "current"
		}
		page.Cycles = append(page.Cycles, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return CyclePage{}, err
	}
	return page, tx.Commit()
}
func (s *Service) GetCycle(ctx context.Context, id int64) (Cycle, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Cycle{}, err
	}
	query, args := viewerSQL(ctx, `SELECT t.key FROM cycles c JOIN teams t ON t.id=c.team_id WHERE c.id=? AND t.workspace_id=? AND t.id IN (`+visibleTeamIDs+`)`, id, s.workspace)
	var key string
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&key); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Cycle{}, ErrCycleNotFound
		}
		return Cycle{}, err
	}
	page, err := s.ListCycles(ctx, key)
	if err != nil {
		return Cycle{}, err
	}
	for _, c := range page.Cycles {
		if c.ID == id {
			return c, nil
		}
	}
	return Cycle{}, ErrCycleNotFound
}
func (s *Service) UpdateCycle(ctx context.Context, in UpdateCycle) (Cycle, error) {
	if _, err := s.GetCycle(ctx, in.ID); err != nil {
		return Cycle{}, err
	}
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE cycles SET name=COALESCE(?,name),description=COALESCE(?,description) WHERE id=?`, in.Name, in.Description, in.ID)
	if err != nil {
		return Cycle{}, err
	}
	return s.GetCycle(ctx, in.ID)
}
func (s *Service) validateIssueCycle(ctx context.Context, tx *sql.Tx, id *int64, team int64, current sql.NullInt64) error {
	if id == nil {
		return nil
	}
	if *id < 0 {
		return ErrCycleReference
	}
	if *id == 0 || (current.Valid && current.Int64 == *id) {
		return nil
	}
	var enabled bool
	var completed sql.NullString
	var end string
	err := tx.QueryRowContext(ctx, `SELECT s.enabled,c.completed_at,c.ends_at FROM cycles c JOIN team_cycle_settings s ON s.team_id=c.team_id WHERE c.id=? AND c.team_id=?`, *id, team).Scan(&enabled, &completed, &end)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCycleNotFound
	}
	if err != nil {
		return err
	}
	if !enabled || completed.Valid || end <= cycleTime(s.now()) {
		return ErrCycleClosed
	}
	return nil
}
